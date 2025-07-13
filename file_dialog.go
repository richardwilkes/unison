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
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xfilepath"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
)

const pathSeparator = string(os.PathSeparator)

// FileDialog represents the common API for open and save dialogs.
type FileDialog interface {
	// InitialDirectory returns a path pointing to the directory the dialog will open up in.
	InitialDirectory() string
	// SetInitialDirectory sets the directory the dialog will open up in.
	SetInitialDirectory(dir string)
	// AllowedExtensions returns the set of permitted file extensions. nil will be returned if all files are allowed.
	AllowedExtensions() []string
	// SetAllowedExtensions sets the permitted file extensions that may be selected. Just the extension is needed, e.g.
	// "txt", not ".txt" or "*.txt", etc. Pass in nil to allow all files.
	SetAllowedExtensions(extensions ...string)
	// RunModal displays the dialog, allowing the user to make a selection. Returns true if successful or false if
	// canceled.
	RunModal() bool
	// Path returns the path that was chosen.
	Path() string
}

type fileDialog struct {
	dialog         *Dialog
	parentDirPopup *PopupMenu[*parentDirItem]
	fileNameField  *Field
	scroller       *ScrollPanel
	fileList       *List[*fileListItem]
	filterPopup    *PopupMenu[string]
	readable       []string
	dirEntries     []os.DirEntry
	currentDir     string
	currentExt     string
	fileCommon
	forOpen bool
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
	if d.fileNameField != nil {
		d.fileNameFieldModified(nil, nil)
	}
	if dlg.RunModal() == ModalResponseOK {
		if !d.forOpen {
			_, ok := ValidateSaveFilePath(d.Path(), "", true)
			return ok
		}
		return true
	}
	return false
}

func (d *fileDialog) createContent() *Panel {
	d.readable = make([]string, 0, len(d.extensions))
	for _, ext := range d.extensions {
		if ext != "*" {
			d.readable = append(d.readable, ext)
		}
	}
	switch len(d.readable) {
	case 0:
		d.currentExt = "*"
	case 1:
		d.currentExt = d.readable[0]
	default:
		d.currentExt = strings.Join(d.readable, ";")
	}
	d.prepareCurrentDir(d.initialDir)

	content := NewPanel()
	content.SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})

	d.parentDirPopup = NewPopupMenu[*parentDirItem]()
	d.parentDirPopup.SelectionChangedCallback = d.parentDirPopupSelectionHandler
	d.rebuildParentDirs()
	content.AddChild(d.parentDirPopup)
	d.parentDirPopup.SetLayoutData(&FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: align.Middle,
		VAlign: align.Middle,
		HGrab:  true,
	})

	if !d.forOpen {
		d.fileNameField = NewField()
		d.fileNameField.SetText(d.initialName)
		d.fileNameField.ModifiedCallback = d.fileNameFieldModified
		d.fileNameField.KeyDownCallback = d.fileNameFieldKeyDown
		content.AddChild(d.fileNameField)
		d.fileNameField.SetLayoutData(&FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: align.Fill,
			VAlign: align.Middle,
			HGrab:  true,
		})
	}

	d.fileList = NewList[*fileListItem]().SetAllowMultipleSelection(d.allowMultipleSelection)
	d.fileList.NewSelectionCallback = d.fileListSelectionHandler
	d.fileList.DoubleClickCallback = d.fileListDoubleClickHandler
	d.rebuildFileList()
	d.scroller = NewScrollPanel()
	d.scroller.SetBorder(NewLineBorder(ThemeSurfaceEdge, 0, geom.NewUniformInsets(1), false))
	d.scroller.SetContent(d.fileList, behavior.Follow, behavior.Fill)
	content.AddChild(d.scroller)
	d.scroller.SetLayoutData(&FlexLayoutData{
		MinSize: geom.Size{Width: 300, Height: 200},
		HSpan:   1,
		VSpan:   1,
		HAlign:  align.Fill,
		VAlign:  align.Fill,
		HGrab:   true,
		VGrab:   true,
	})

	if len(d.extensions) > 1 {
		d.filterPopup = NewPopupMenu[string]()
		d.filterPopup.AddItem(i18n.Text("All Readable Files"))
		for _, ext := range d.extensions {
			if ext == "*" {
				d.filterPopup.AddItem(i18n.Text("All Files"))
			} else {
				d.filterPopup.AddItem("*." + ext)
			}
		}
		d.filterPopup.SelectionChangedCallback = d.filterHandler
		d.filterPopup.SelectIndex(0)
		content.AddChild(d.filterPopup)
		d.filterPopup.SetLayoutData(&FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: align.Middle,
			VAlign: align.Middle,
			HGrab:  true,
		})
	}

	return content
}

func (d *fileDialog) rebuildParentDirs() {
	d.parentDirPopup.RemoveAllItems()
	dir := d.currentDir
	vol := filepath.VolumeName(dir)
	for {
		parent, f := filepath.Split(dir)
		if dir == pathSeparator || dir == vol {
			f = pathSeparator
		}
		d.parentDirPopup.AddItem(&parentDirItem{
			name: f,
			path: dir,
		})
		if dir == pathSeparator || dir == vol {
			break
		}
		if parent != pathSeparator && parent != vol {
			parent = strings.TrimRight(parent, pathSeparator)
		}
		if parent == "" || parent == vol {
			break
		}
		dir = parent
	}
	d.parentDirPopup.SelectIndex(0)
	d.parentDirPopup.MarkForLayoutAndRedraw()
	if p := d.parentDirPopup.Parent(); p != nil {
		p.NeedsLayout = true
	}
}

func (d *fileDialog) parentDirPopupSelectionHandler(popup *PopupMenu[*parentDirItem]) {
	if popup.SelectedIndex() > 0 {
		if item, ok := popup.Selected(); ok {
			d.changeDirTo(item.path)
		}
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
		item := d.fileList.DataAtIndex(i)
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
		i = d.fileList.Selection.NextSet(i + 1)
	}
	d.dialog.Button(ModalResponseOK).SetEnabled(okEnabled)
}

func (d *fileDialog) fileListDoubleClickHandler() {
	i := d.fileList.Anchor()
	if i == -1 {
		i = d.fileList.Selection.FirstSet()
	}
	if i != -1 {
		if item := d.fileList.DataAtIndex(i); item.entry.IsDir() {
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

func (d *fileDialog) fileNameFieldModified(_, _ *FieldState) {
	text := d.fileNameField.Text()
	if text != "" {
		switch {
		case d.currentExt == "*":
		case strings.Contains(d.currentExt, ";"):
			found := false
			ext := filepath.Ext(text)
			if ext != "" {
				ext = ext[1:]
				for _, one := range d.readable {
					if strings.EqualFold(ext, one) {
						found = true
						break
					}
				}
			}
			if !found {
				text = xfilepath.TrimExtension(text) + "." + d.readable[0]
			}
		default:
			text = xfilepath.TrimExtension(text) + "." + d.currentExt
		}
	}
	d.paths = []string{filepath.Join(d.currentDir, text)}
	d.dialog.Button(ModalResponseOK).SetEnabled(text != "")
}

func (d *fileDialog) rebuildFileList() {
	d.fileList.RemoveRange(0, d.fileList.Count()-1)
	for _, entry := range d.dirEntries {
		if !entry.IsDir() {
			ext := filepath.Ext(entry.Name())
			switch {
			case d.currentExt == "*":
			case ext == "":
				continue
			case strings.Contains(d.currentExt, ";"):
				ext = ext[1:]
				found := false
				for _, one := range d.readable {
					if strings.EqualFold(ext, one) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			default:
				if !strings.EqualFold(ext[1:], d.currentExt) {
					continue
				}
			}
		}
		d.fileList.Append(&fileListItem{entry: entry})
	}
	if d.fileList.Parent() != nil {
		_, pref, _ := d.fileList.Sizes(geom.Size{})
		d.fileList.SetFrameRect(geom.Rect{Size: pref})
		d.scroller.SetPosition(0, 0)
	}
}

func (d *fileDialog) filterHandler(popup *PopupMenu[string]) {
	index := popup.SelectedIndex()
	if index == 0 {
		d.currentExt = strings.Join(d.readable, ";")
	} else {
		d.currentExt = d.extensions[index-1]
	}
	d.rebuildFileList()
}

func (d *fileDialog) prepareCurrentDir(dir string) {
	d.currentDir = resolveToAcceptableAbsDir(dir)
	var err error
	if d.dirEntries, err = os.ReadDir(d.currentDir); err != nil {
		errs.Log(err, "path", d.currentDir)
	}
}

type fileCommon struct {
	initialDir             string
	initialName            string
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
	fc.initialName = i18n.Text("untitled")
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

func (fc *fileCommon) InitialFileName() string {
	return fc.initialName
}

func (fc *fileCommon) SetInitialFileName(name string) {
	fc.initialName = name
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
	if d, err := filepath.Abs(dir); err == nil && xos.IsDir(d) {
		return d
	}
	return ""
}
