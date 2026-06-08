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
	"net/url"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// IIDs for the COM interfaces we implement.
var (
	iidIUnknown    = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIDropTarget = windows.GUID{Data1: 0x00000122, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIDropSource = windows.GUID{Data1: 0x00000121, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIDataObject = windows.GUID{Data1: 0x0000010E, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// ======================== winDragInfo ========================

// winDragInfo implements drag.Info using a Windows IDataObject.
type winDragInfo struct {
	obj    *w32.IDataObject
	opMask drag.Op
}

var _ drag.Info = (*winDragInfo)(nil)

func (d *winDragInfo) SourceDragOpMask() drag.Op { return d.opMask }

func (d *winDragInfo) DataTypes() []string {
	enum := d.obj.EnumFormatEtc(1) // DATADIR_GET = 1
	if enum == nil {
		return nil
	}
	defer enum.Release()
	var result []string
	batch := make([]w32.FORMATETC, 16)
	for {
		n := enum.Next(batch)
		if n == 0 {
			break
		}
		for _, fe := range batch[:n] {
			name := w32ReverseDataType(w32.ClipboardFormat(fe.CfFormat))
			if name != "" && !containsString(result, name) {
				result = append(result, name)
			}
		}
	}
	return result
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func (d *winDragInfo) HasString() bool {
	return d.hasFormat(w32.CFUnicodeText)
}

func (d *winDragInfo) HasFilePaths() bool {
	return d.hasFormat(w32.CFHDrop)
}

func (d *winDragInfo) HasURLs() bool {
	return d.hasFormat(w32.CFHDrop) || d.hasDataType(uti.URL.UTI)
}

func (d *winDragInfo) HasDataType(dataType string) bool {
	cf := w32LookupDataType(dataType)
	if cf == w32.CFNone {
		return false
	}
	return d.hasFormat(cf)
}

func (d *winDragInfo) Text() string {
	data := d.getFormatData(w32.CFUnicodeText)
	if len(data) < 2 {
		return ""
	}
	// Strip null terminator(s)
	u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&data[0])), len(data)/2)
	end := len(u16)
	for end > 0 && u16[end-1] == 0 {
		end--
	}
	return windows.UTF16ToString(u16[:end])
}

func (d *winDragInfo) FilePaths() []string {
	data := d.getFormatData(w32.CFHDrop)
	return parseDropFiles(data)
}

func (d *winDragInfo) URLs() []*url.URL {
	var result []*url.URL
	// File paths from CF_HDROP as file:// URLs
	for _, path := range d.FilePaths() {
		// Convert backslashes and build file:// URL
		urlPath := "/" + strings.ReplaceAll(path, "\\", "/")
		u, err := url.Parse("file://" + urlPath)
		if err == nil {
			result = append(result, u)
		}
	}
	// Generic URL data type
	if raw := d.Data(uti.URL.UTI); len(raw) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if u, err := url.Parse(line); err == nil {
				result = append(result, u)
			}
		}
	}
	return result
}

func (d *winDragInfo) Data(dataType string) []byte {
	cf := w32LookupDataType(dataType)
	if cf == w32.CFNone {
		return nil
	}
	raw := d.getFormatData(cf)
	if len(raw) == 0 {
		return nil
	}
	if uti.UTF8PlainText.ConformsTo(uti.ByUTI(dataType)) {
		// Convert from UTF-16LE to UTF-8
		if len(raw) < 2 {
			return nil
		}
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&raw[0])), len(raw)/2)
		end := len(u16)
		for end > 0 && u16[end-1] == 0 {
			end--
		}
		return []byte(windows.UTF16ToString(u16[:end]))
	}
	return raw
}

func (d *winDragInfo) hasFormat(cf w32.ClipboardFormat) bool {
	fe := w32.FORMATETC{
		CfFormat: uint16(cf),
		DwAspect: w32.DVAspectContent,
		Lindex:   -1,
		Tymed:    w32.TyMedHGlobal,
	}
	return d.obj.QueryGetData(&fe) == w32.COM_S_OK
}

func (d *winDragInfo) hasDataType(utiStr string) bool {
	cf := w32LookupDataType(utiStr)
	if cf == w32.CFNone {
		return false
	}
	return d.hasFormat(cf)
}

func (d *winDragInfo) getFormatData(cf w32.ClipboardFormat) []byte {
	fe := w32.FORMATETC{
		CfFormat: uint16(cf),
		DwAspect: w32.DVAspectContent,
		Lindex:   -1,
		Tymed:    w32.TyMedHGlobal,
	}
	stg, r := d.obj.GetData(&fe)
	if r != w32.COM_S_OK {
		return nil
	}
	defer w32.ReleaseStgMedium(&stg)
	if stg.Tymed != w32.TyMedHGlobal || stg.Data == 0 {
		return nil
	}
	h := syscall.Handle(stg.Data)
	buf := w32.GlobalLock(h)
	if buf == 0 {
		return nil
	}
	defer w32.GlobalUnlock(h)
	size := w32.GlobalSize(h)
	if size == 0 {
		return nil
	}
	data := make([]byte, size)
	copy(data, unsafe.Slice((*byte)(unsafe.Pointer(buf)), size))
	return data
}

// parseDropFiles extracts file paths from a raw CF_HDROP HGLOBAL buffer.
func parseDropFiles(data []byte) []string {
	if len(data) < int(unsafe.Sizeof(w32.DROPFILES{})) {
		return nil
	}
	df := (*w32.DROPFILES)(unsafe.Pointer(&data[0]))
	if df.FWide == 0 {
		return nil // Only support Unicode (FWide=TRUE)
	}
	offset := int(df.PFiles)
	if offset >= len(data) {
		return nil
	}
	// The file list is a double-null-terminated list of wide strings.
	remaining := data[offset:]
	if len(remaining)%2 != 0 {
		remaining = remaining[:len(remaining)-1]
	}
	u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&remaining[0])), len(remaining)/2)
	var paths []string
	start := 0
	for i, v := range u16 {
		if v == 0 {
			if i == start {
				break // Double null: end of list
			}
			paths = append(paths, windows.UTF16ToString(u16[start:i]))
			start = i + 1
		}
	}
	return paths
}

// w32ReverseDataType returns the UTI string for a Windows clipboard format, or "".
func w32ReverseDataType(cf w32.ClipboardFormat) string {
	w32DataTypeMapLock.RLock()
	name, ok := w32ReverseDataTypeMap[cf]
	w32DataTypeMapLock.RUnlock()
	if ok {
		return name
	}
	if name = w32.GetClipboardFormatNameW(cf); name != "" {
		return name
	}
	return ""
}

// ======================== winDropTarget (implements IDropTarget) ========================

// winDropTarget is a Go-implemented COM IDropTarget registered with a Window.
type winDropTarget struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dropTargetVtbl
	window   *Window
	refCount int32
	pinner   runtime.Pinner
	opMask   drag.Op          // source's allowed ops, set in DragEnter
	dataObj  *w32.IDataObject // current drag's data object (valid between DragEnter and DragLeave/Drop)
}

var dropTargetVtbl [7]uintptr

func init() {
	dropTargetVtbl[0] = windows.NewCallback(dropTargetQueryInterface)
	dropTargetVtbl[1] = windows.NewCallback(dropTargetAddRef)
	dropTargetVtbl[2] = windows.NewCallback(dropTargetRelease)
	dropTargetVtbl[3] = windows.NewCallback(dropTargetDragEnter)
	dropTargetVtbl[4] = windows.NewCallback(dropTargetDragOver)
	dropTargetVtbl[5] = windows.NewCallback(dropTargetDragLeave)
	dropTargetVtbl[6] = windows.NewCallback(dropTargetDrop)
}

func newWinDropTarget(w *Window) *winDropTarget {
	dt := &winDropTarget{
		window:   w,
		refCount: 1,
	}
	dt.lpVtbl = uintptr(unsafe.Pointer(&dropTargetVtbl[0]))
	dt.pinner.Pin(dt)
	return dt
}

func (dt *winDropTarget) revoke() {
	if dt == nil {
		return
	}
	dt.dataObj = nil
	dt.pinner.Unpin()
}

func dropTargetQueryInterface(this, riid, ppvObject uintptr) uintptr {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidIUnknown || *guid == iidIDropTarget {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		dropTargetAddRef(this)
		return w32.COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return w32.COM_E_NOINTERFACE
}

func dropTargetAddRef(this uintptr) uintptr {
	dt := (*winDropTarget)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&dt.refCount, 1))
}

func dropTargetRelease(this uintptr) uintptr {
	dt := (*winDropTarget)(unsafe.Pointer(this))
	n := atomic.AddInt32(&dt.refCount, -1)
	return uintptr(n)
}

func dropTargetDragEnter(this, pDataObj uintptr, grfKeyState w32.MKDnD, pt uintptr, pdwEffect *w32.DropEffect) uintptr {
	dt := (*winDropTarget)(unsafe.Pointer(this))
	dt.opMask = dropEffectToOp(*pdwEffect)
	dt.dataObj = (*w32.IDataObject)(unsafe.Pointer(pDataObj))
	dt.dataObj.AddRef()
	info := &winDragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.dragEntered(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	*pdwEffect = opToDropEffect(op)
	return w32.COM_S_OK
}

func dropTargetDragOver(this uintptr, grfKeyState w32.MKDnD, pt uintptr, pdwEffect *w32.DropEffect) uintptr {
	dt := (*winDropTarget)(unsafe.Pointer(this))
	if dt.dataObj == nil {
		*pdwEffect = w32.DropEffectNone
		return w32.COM_S_OK
	}
	info := &winDragInfo{obj: dt.dataObj, opMask: dt.opMask}
	op := dt.window.dragUpdate(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	*pdwEffect = opToDropEffect(op)
	return w32.COM_S_OK
}

func dropTargetDragLeave(this uintptr) uintptr {
	dt := (*winDropTarget)(unsafe.Pointer(this))
	if dt.dataObj != nil {
		dt.dataObj.Release()
		dt.dataObj = nil
	}
	dt.window.dragExit()
	return w32.COM_S_OK
}

func dropTargetDrop(this, pDataObj uintptr, grfKeyState w32.MKDnD, pt, pdwEffect uintptr) uintptr {
	dt := (*winDropTarget)(unsafe.Pointer(this))
	if dt.dataObj != nil {
		dt.dataObj.Release()
		dt.dataObj = nil
	}
	dataObj := (*w32.IDataObject)(unsafe.Pointer(pDataObj))
	info := &winDragInfo{obj: dataObj, opMask: dt.opMask}
	dt.window.drop(info, dropTargetClientPt(dt.window, pt), dropKeyStateMods(grfKeyState))
	*(*w32.DropEffect)(unsafe.Pointer(pdwEffect)) = w32.DropEffectNone
	return w32.COM_S_OK
}

func dropTargetClientPt(w *Window, pt uintptr) geom.Point {
	x := int32(pt & 0xFFFFFFFF)
	y := int32(pt >> 32)
	var screenPt w32.POINT
	screenPt.X = x
	screenPt.Y = y
	w32.ScreenToClient(w.wnd.wnd, &screenPt)
	return w.apiConvertRawMouse(geom.NewPoint(float32(screenPt.X), float32(screenPt.Y)))
}

func dropKeyStateMods(grfKeyState w32.MKDnD) mod.Modifiers {
	var mods mod.Modifiers
	if grfKeyState&w32.MKDnDShift != 0 {
		mods |= mod.Shift
	}
	if grfKeyState&w32.MKDnDControl != 0 {
		mods |= mod.Control
	}
	if grfKeyState&w32.MKDnDAlt != 0 {
		mods |= mod.Option
	}
	return mods
}

func dropEffectToOp(effect w32.DropEffect) drag.Op {
	var op drag.Op
	if effect&w32.DropEffectCopy != 0 {
		op |= drag.Copy
	}
	if effect&w32.DropEffectMove != 0 {
		op |= drag.Move
	}
	return op
}

func opToDropEffect(op drag.Op) w32.DropEffect {
	switch {
	case op&drag.Copy != 0:
		return w32.DropEffectCopy
	case op&drag.Move != 0:
		return w32.DropEffectMove
	default:
		return w32.DropEffectNone
	}
}

func opMaskToDropEffect(op drag.Op) w32.DropEffect {
	var effect w32.DropEffect
	if op&drag.Copy != 0 {
		effect |= w32.DropEffectCopy
	}
	if op&drag.Move != 0 {
		effect |= w32.DropEffectMove
	}
	return effect
}

// ======================== winDropSource (implements IDropSource) ========================

// winDropSource is a Go-implemented COM IDropSource used when initiating a drag.
type winDropSource struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dropSrcVtbl
	refCount int32
}

var dropSrcVtbl [5]uintptr

func init() {
	dropSrcVtbl[0] = windows.NewCallback(dropSrcQueryInterface)
	dropSrcVtbl[1] = windows.NewCallback(dropSrcAddRef)
	dropSrcVtbl[2] = windows.NewCallback(dropSrcRelease)
	dropSrcVtbl[3] = windows.NewCallback(dropSrcQueryContinueDrag)
	dropSrcVtbl[4] = windows.NewCallback(dropSrcGiveFeedback)
}

func newWinDropSource() *winDropSource {
	src := &winDropSource{refCount: 1}
	src.lpVtbl = uintptr(unsafe.Pointer(&dropSrcVtbl[0]))
	return src
}

func dropSrcQueryInterface(this, riid, ppvObject uintptr) uintptr {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidIUnknown || *guid == iidIDropSource {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		dropSrcAddRef(this)
		return w32.COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return w32.COM_E_NOINTERFACE
}

func dropSrcAddRef(this uintptr) uintptr {
	src := (*winDropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&src.refCount, 1))
}

func dropSrcRelease(this uintptr) uintptr {
	src := (*winDropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&src.refCount, -1))
}

// dropSrcQueryContinueDrag is called repeatedly during a drag to check whether to continue.
// fEscapePressed: non-zero if Escape was pressed; grfKeyState: current mouse/key state.
func dropSrcQueryContinueDrag(this, fEscapePressed uintptr, grfKeyState w32.MKDnD) uintptr {
	if fEscapePressed != 0 {
		return w32.COM_DRAGDROP_S_CANCEL
	}
	// Drop when the left mouse button is released (not held in grfKeyState).
	if grfKeyState&w32.MKDnDLButton == 0 {
		return w32.COM_DRAGDROP_S_DROP
	}
	return w32.COM_S_OK
}

func dropSrcGiveFeedback(_ uintptr, _ uintptr) uintptr {
	return w32.COM_DRAGDROP_S_USEDEFAULTCURSORS
}

// ======================== winDataObject (implements IDataObject) ========================

type dragDataEntry struct {
	fmtEtc w32.FORMATETC
	data   []byte // data in Windows format (UTF-16LE for text, raw bytes otherwise)
}

// winDataObject is a Go-implemented COM IDataObject that carries drag data.
type winDataObject struct {
	lpVtbl   uintptr // MUST BE FIRST: points to dataObjVtbl
	entries  []dragDataEntry
	enumFmt  *winEnumFORMATETC
	refCount int32
}

var dataObjVtbl [12]uintptr

func init() {
	dataObjVtbl[0] = windows.NewCallback(dataObjQueryInterface)
	dataObjVtbl[1] = windows.NewCallback(dataObjAddRef)
	dataObjVtbl[2] = windows.NewCallback(dataObjRelease)
	dataObjVtbl[3] = windows.NewCallback(dataObjGetData)
	dataObjVtbl[4] = windows.NewCallback(dataObjGetDataHere)
	dataObjVtbl[5] = windows.NewCallback(dataObjQueryGetData)
	dataObjVtbl[6] = windows.NewCallback(dataObjGetCanonicalFormatEtc)
	dataObjVtbl[7] = windows.NewCallback(dataObjSetData)
	dataObjVtbl[8] = windows.NewCallback(dataObjEnumFormatEtc)
	dataObjVtbl[9] = windows.NewCallback(dataObjDAdvise)
	dataObjVtbl[10] = windows.NewCallback(dataObjDUnadvise)
	dataObjVtbl[11] = windows.NewCallback(dataObjEnumDAdvise)
}

func newWinDataObject(data []drag.Data, opMask drag.Op) *winDataObject {
	entries := make([]dragDataEntry, 0, len(data))
	for _, d := range data {
		cf := w32LookupDataType(d.Type.UTI)
		if cf == w32.CFNone {
			continue
		}
		var raw []byte
		if uti.UTF8PlainText.ConformsTo(d.Type) {
			s, err := windows.UTF16FromString(string(d.Data))
			if err != nil {
				errs.Log(err)
				continue
			}
			raw = make([]byte, len(s)*2)
			for i, v := range s {
				raw[i*2] = byte(v)
				raw[i*2+1] = byte(v >> 8)
			}
		} else {
			raw = d.Data
		}
		entries = append(entries, dragDataEntry{
			fmtEtc: w32.FORMATETC{
				CfFormat: uint16(cf),
				DwAspect: w32.DVAspectContent,
				Lindex:   -1,
				Tymed:    w32.TyMedHGlobal,
			},
			data: raw,
		})
	}
	obj := &winDataObject{entries: entries, refCount: 1}
	obj.lpVtbl = uintptr(unsafe.Pointer(&dataObjVtbl[0]))
	obj.enumFmt = newWinEnumFORMATETC(obj)
	return obj
}

func (obj *winDataObject) findEntry(cf uint16) ([]byte, bool) {
	for _, e := range obj.entries {
		if e.fmtEtc.CfFormat == cf {
			return e.data, true
		}
	}
	return nil, false
}

func dataObjQueryInterface(this, riid, ppvObject uintptr) uintptr {
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidIUnknown || *guid == iidIDataObject {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		dataObjAddRef(this)
		return w32.COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return w32.COM_E_NOINTERFACE
}

func dataObjAddRef(this uintptr) uintptr {
	obj := (*winDataObject)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&obj.refCount, 1))
}

func dataObjRelease(this uintptr) uintptr {
	obj := (*winDataObject)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&obj.refCount, -1))
}

func dataObjGetData(this, pformatetcIn, pmedium uintptr) uintptr {
	obj := (*winDataObject)(unsafe.Pointer(this))
	fe := (*w32.FORMATETC)(unsafe.Pointer(pformatetcIn))
	data, ok := obj.findEntry(fe.CfFormat)
	if !ok {
		return w32.COM_DV_E_FORMATETC
	}
	if fe.Tymed&w32.TyMedHGlobal == 0 {
		return w32.COM_DV_E_TYMED
	}
	h := w32.GlobalAlloc(w32.GMemMoveable, len(data))
	if h == 0 {
		return w32.COM_E_NOTIMPL
	}
	buf := w32.GlobalLock(h)
	if buf == 0 {
		w32.GlobalFree(h)
		return w32.COM_E_NOTIMPL
	}
	copy(unsafe.Slice((*byte)(unsafe.Pointer(buf)), len(data)), data)
	w32.GlobalUnlock(h)
	stg := (*w32.STGMEDIUM)(unsafe.Pointer(pmedium))
	stg.Tymed = w32.TyMedHGlobal
	stg.Data = uintptr(h)
	stg.PUnkForRelease = 0
	return w32.COM_S_OK
}

func dataObjGetDataHere(_, _, _ uintptr) uintptr { return w32.COM_E_NOTIMPL }

func dataObjQueryGetData(this, pformatetc uintptr) uintptr {
	obj := (*winDataObject)(unsafe.Pointer(this))
	fe := (*w32.FORMATETC)(unsafe.Pointer(pformatetc))
	_, ok := obj.findEntry(fe.CfFormat)
	if !ok {
		return w32.COM_DV_E_FORMATETC
	}
	if fe.Tymed&w32.TyMedHGlobal == 0 {
		return w32.COM_DV_E_TYMED
	}
	return w32.COM_S_OK
}

func dataObjGetCanonicalFormatEtc(_, _, pformatetcOut uintptr) uintptr {
	// Indicate we don't canonicalize.
	(*w32.FORMATETC)(unsafe.Pointer(pformatetcOut)).Ptd = 0
	return w32.COM_DATA_S_SAMEFORMATETC
}

func dataObjSetData(_, _, _, _ uintptr) uintptr { return w32.COM_E_NOTIMPL }

func dataObjEnumFormatEtc(this, dwDirection, ppenumFormatetc uintptr) uintptr {
	if dwDirection != 1 { // DATADIR_GET = 1
		return w32.COM_E_NOTIMPL
	}
	obj := (*winDataObject)(unsafe.Pointer(this))
	obj.enumFmt.Reset()
	enumAddRef(uintptr(unsafe.Pointer(obj.enumFmt)))
	*(*uintptr)(unsafe.Pointer(ppenumFormatetc)) = uintptr(unsafe.Pointer(obj.enumFmt))
	return w32.COM_S_OK
}

func dataObjDAdvise(_, _, _, _, _ uintptr) uintptr { return w32.COM_OLE_E_ADVISENOTSUPPORTED }
func dataObjDUnadvise(_, _ uintptr) uintptr        { return w32.COM_OLE_E_ADVISENOTSUPPORTED }
func dataObjEnumDAdvise(_, _ uintptr) uintptr      { return w32.COM_OLE_E_ADVISENOTSUPPORTED }

// ======================== winEnumFORMATETC (implements IEnumFORMATETC) ========================

// winEnumFORMATETC is a Go-implemented COM IEnumFORMATETC for a winDataObject.
type winEnumFORMATETC struct {
	lpVtbl   uintptr // MUST BE FIRST: points to enumFmtVtbl
	obj      *winDataObject
	pos      int
	refCount int32
}

var enumFmtVtbl [7]uintptr

func init() {
	enumFmtVtbl[0] = windows.NewCallback(enumQueryInterface)
	enumFmtVtbl[1] = windows.NewCallback(enumAddRef)
	enumFmtVtbl[2] = windows.NewCallback(enumRelease)
	enumFmtVtbl[3] = windows.NewCallback(enumNext)
	enumFmtVtbl[4] = windows.NewCallback(enumSkip)
	enumFmtVtbl[5] = windows.NewCallback(enumResetCB)
	enumFmtVtbl[6] = windows.NewCallback(enumClone)
}

func newWinEnumFORMATETC(obj *winDataObject) *winEnumFORMATETC {
	e := &winEnumFORMATETC{obj: obj, refCount: 1}
	e.lpVtbl = uintptr(unsafe.Pointer(&enumFmtVtbl[0]))
	return e
}

func (e *winEnumFORMATETC) Reset() {
	e.pos = 0
}

func enumQueryInterface(this, riid, ppvObject uintptr) uintptr {
	// IEnumFORMATETC IID: {00000103-0000-0000-C000-000000000046}
	iidIEnumFORMATETC := windows.GUID{Data1: 0x00000103, Data2: 0, Data3: 0, Data4: [8]byte{0xC0, 0, 0, 0, 0, 0, 0, 0x46}}
	guid := (*windows.GUID)(unsafe.Pointer(riid))
	if *guid == iidIUnknown || *guid == iidIEnumFORMATETC {
		*(*uintptr)(unsafe.Pointer(ppvObject)) = this
		enumAddRef(this)
		return w32.COM_S_OK
	}
	*(*uintptr)(unsafe.Pointer(ppvObject)) = 0
	return w32.COM_E_NOINTERFACE
}

func enumAddRef(this uintptr) uintptr {
	e := (*winEnumFORMATETC)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&e.refCount, 1))
}

func enumRelease(this uintptr) uintptr {
	e := (*winEnumFORMATETC)(unsafe.Pointer(this))
	return uintptr(atomic.AddInt32(&e.refCount, -1))
}

func enumNext(this, celt, rgelt, pceltFetched uintptr) uintptr {
	e := (*winEnumFORMATETC)(unsafe.Pointer(this))
	count := int(celt)
	dst := unsafe.Slice((*w32.FORMATETC)(unsafe.Pointer(rgelt)), count)
	fetched := 0
	for fetched < count && e.pos < len(e.obj.entries) {
		dst[fetched] = e.obj.entries[e.pos].fmtEtc
		fetched++
		e.pos++
	}
	if pceltFetched != 0 {
		*(*uint32)(unsafe.Pointer(pceltFetched)) = uint32(fetched)
	}
	if fetched == count {
		return w32.COM_S_OK
	}
	return w32.COM_S_FALSE
}

func enumSkip(this, celt uintptr) uintptr {
	e := (*winEnumFORMATETC)(unsafe.Pointer(this))
	count := int(celt)
	remaining := len(e.obj.entries) - e.pos
	if count > remaining {
		e.pos = len(e.obj.entries)
		return w32.COM_S_FALSE
	}
	e.pos += count
	return w32.COM_S_OK
}

func enumResetCB(this uintptr) uintptr {
	e := (*winEnumFORMATETC)(unsafe.Pointer(this))
	e.pos = 0
	return w32.COM_S_OK
}

func enumClone(_ uintptr, _ uintptr) uintptr { return w32.COM_E_NOTIMPL }
