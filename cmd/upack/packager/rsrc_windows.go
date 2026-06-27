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
	"bytes"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"maps"
	"os"
	"slices"
	"sort"
	"strings"
	"text/template"
	"unicode/utf16"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/image/draw"
)

// Standard resource type IDs.
//
// https://docs.microsoft.com/en-us/windows/win32/menurc/resource-types
const (
	rtIcon      uint16 = 3
	rtGroupIcon uint16 = 14
	rtVersion   uint16 = 16
	rtManifest  uint16 = 24
)

// Language Code Identifiers used by this package.
const (
	lcidNeutral uint16 = 0
	lcidDefault uint16 = 0x409 // en-US, the default in most tools and APIs
)

type identifier struct {
	name   string
	id     uint16
	isName bool
}

func idNum(id uint16) identifier {
	return identifier{id: id}
}

func idName(name string) identifier {
	return identifier{name: name, isName: true}
}

func (i identifier) lessThan(other identifier) bool {
	if i.isName != other.isName {
		return i.isName
	}
	if i.isName {
		return i.name < other.name
	}
	return i.id < other.id
}

func checkIdentifier(ident identifier) error {
	if ident.isName {
		if ident.name == "" {
			return errs.New("resource name cannot be empty")
		}
		if strings.ContainsRune(ident.name, 0) {
			return errs.New("resource name cannot contain a NUL")
		}
		return nil
	}
	if ident.id == 0 {
		return errs.New("resource id cannot be zero")
	}
	return nil
}

// resourceSet collects resources and writes them as a COFF object file.
//
// https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#the-rsrc-section
type resourceSet struct {
	types      map[identifier]*typeEntry
	lastIconID uint16
}

func (rs *resourceSet) set(typeID, resID identifier, langID uint16, data []byte) {
	if rs.types == nil {
		rs.types = make(map[identifier]*typeEntry)
	}
	te := rs.types[typeID]
	if te == nil {
		te = &typeEntry{resources: make(map[identifier]*resourceEntry)}
		rs.types[typeID] = te
	}
	re := te.resources[resID]
	if re == nil {
		te.orderedKeys = nil
		re = &resourceEntry{data: make(map[uint16]*dataEntry)}
		te.resources[resID] = re
	}
	if typeID == idNum(rtIcon) && !resID.isName && rs.lastIconID < resID.id {
		rs.lastIconID = resID.id
	}
	de := re.data[langID]
	if de == nil {
		re.orderedKeys = nil
		de = &dataEntry{}
		re.data[langID] = de
	}
	de.data = data
}

// setManifest embeds an application manifest with the given description. The other settings are fixed to what a Unison
// app needs: Windows 10 or later, run as the invoking user, per-monitor v2 DPI awareness, high-resolution scrolling
// awareness, and long-path awareness.
func (rs *resourceSet) setManifest(description string) {
	rs.set(idNum(rtManifest), idNum(1), lcidDefault, makeManifest(description))
}

// setVersionInfo embeds the version info as a single en-US resource. The OS resource loader falls back to en-US, so
// this is found regardless of the system's UI language (the same as the manifest, which is embedded the same way).
func (rs *resourceSet) setVersionInfo(vi *versionInfo) {
	rs.set(idNum(rtVersion), idNum(1), lcidDefault, vi.bytes())
}

func (rs *resourceSet) setIcon(resID identifier, ic *winIcon) error {
	if err := checkIdentifier(resID); err != nil {
		return err
	}
	ic.order()
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, iconDirHeader{Type: 1, Count: uint16(len(ic.images))})
	for i := range ic.images {
		id := rs.lastIconID + 1
		binary.Write(buf, binary.LittleEndian, iconResDirEntry{iconInfo: ic.images[i].info, ID: id})
		rs.set(idNum(rtIcon), idNum(id), lcidNeutral, ic.images[i].image)
	}
	rs.set(idNum(rtGroupIcon), resID, lcidNeutral, buf.Bytes())
	return nil
}

// COFF object file
//
// https://docs.microsoft.com/en-us/windows/win32/debug/pe-format

const (
	imageSCNMemRead            = 0x00000040
	imageSCNCntInitializedData = 0x40000000
	sizeOfReloc                = 10
	imageSymClassStatic        = 3

	// Image-relative (..._ADDR32NB) relocation type per architecture.
	// https://docs.microsoft.com/en-us/windows/win32/debug/pe-format#type-indicators
	imageRelAMD64Addr32NB uint16 = 0x3
	imageRelARM64Addr32NB uint16 = 0x2
)

// sysoArch describes a Windows build target the packager emits resources for: the GOARCH used in the ".syso" filename
// (which is how the Go toolchain decides whether to link the object for a given build), the COFF machine type, and the
// architecture's image-relative relocation type.
type sysoArch struct {
	goarch    string
	machine   uint16
	relocType uint16
}

// sysoArches lists every Windows target a packaged app may be built for. A separate object is emitted for each so the
// matching one is linked regardless of the GOARCH the subsequent "go build" targets.
var sysoArches = []sysoArch{
	{goarch: "amd64", machine: pe.IMAGE_FILE_MACHINE_AMD64, relocType: imageRelAMD64Addr32NB},
	{goarch: "arm64", machine: pe.IMAGE_FILE_MACHINE_ARM64, relocType: imageRelARM64Addr32NB},
}

func (rs *resourceSet) writeSyso() error {
	for _, arch := range sysoArches {
		if err := rs.writeOneSyso(arch); err != nil {
			return err
		}
	}
	return nil
}

func (rs *resourceSet) writeOneSyso(arch sysoArch) (err error) {
	f, err := os.Create("rsrc_windows_" + arch.goarch + ".syso")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = errs.Wrap(closeErr)
		}
	}()
	if err = rs.writeCOFF(f, arch); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func (rs *resourceSet) writeCOFF(w io.Writer, arch sysoArch) error {
	file := pe.FileHeader{
		Machine:          arch.machine,
		NumberOfSections: 1,
		NumberOfSymbols:  1,
	}
	section := pe.SectionHeader32{
		Name:            [8]byte{'.', 'r', 's', 'r', 'c'},
		Characteristics: imageSCNMemRead | imageSCNCntInitializedData,
	}
	section.PointerToRawData = uint32(binary.Size(file) + binary.Size(section))
	section.SizeOfRawData = uint32(rs.fullSize())
	section.PointerToRelocations = section.PointerToRawData + section.SizeOfRawData
	section.NumberOfRelocations = uint16(rs.numDataEntries())
	file.PointerToSymbolTable = section.PointerToRelocations + uint32(section.NumberOfRelocations)*sizeOfReloc
	if err := binary.Write(w, binary.LittleEndian, file); err != nil {
		return errs.Wrap(err)
	}
	if err := binary.Write(w, binary.LittleEndian, section); err != nil {
		return errs.Wrap(err)
	}
	s := rs.prepare()
	if err := rs.writeTypeDir(w, s); err != nil {
		return err
	}
	if err := rs.writeResDirs(w, s); err != nil {
		return err
	}
	if err := rs.writeLangDirs(w, s); err != nil {
		return err
	}
	s.offset += len(s.namesData) * 2
	if err := rs.writeDataIndex(w, s); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, s.namesData); err != nil {
		return errs.Wrap(err)
	}
	if err := rs.writeData(w, s); err != nil {
		return err
	}
	for _, a := range s.relocAddr {
		if err := binary.Write(w, binary.LittleEndian, &pe.Reloc{
			VirtualAddress: uint32(a),
			Type:           arch.relocType,
		}); err != nil {
			return errs.Wrap(err)
		}
	}
	if err := binary.Write(w, binary.LittleEndian, &pe.COFFSymbol{
		Name:          [8]byte{'.', 'r', 's', 'r', 'c'},
		SectionNumber: 1,
		StorageClass:  imageSymClassStatic,
	}); err != nil {
		return errs.Wrap(err)
	}
	return errs.Wrap(binary.Write(w, binary.LittleEndian, uint32(4)))
}

// Resource directory (.rsrc section content)
//
// https://docs.microsoft.com/en-us/previous-versions/ms809762(v=msdn.10)#pe-file-resources

const (
	sizeOfDirTable  = 16
	sizeOfDirEntry  = 8
	sizeOfDataEntry = 16
	dataAlignment   = 8 // Visual C++ pads resource data to 8 bytes
	subDirFlag      = 0x80000000
	nameFlag        = 0x80000000
)

type rsrcState struct {
	relocAddr   []int
	nameOffset  map[string]int
	namesData   []uint16
	orderedKeys []identifier
	offset      int
	namesCount  int
}

// prepare calculates the names index and orders all identifiers.
func (rs *resourceSet) prepare() *rsrcState {
	nameSet := make(map[string]struct{})
	s := &rsrcState{nameOffset: make(map[string]int)}
	s.orderedKeys = make([]identifier, 0, len(rs.types))
	for ident, te := range rs.types {
		s.orderedKeys = append(s.orderedKeys, ident)
		te.order()
	}
	sort.Slice(s.orderedKeys, func(i, j int) bool {
		return s.orderedKeys[i].lessThan(s.orderedKeys[j])
	})
	s.namesCount = sort.Search(len(s.orderedKeys), func(i int) bool {
		return !s.orderedKeys[i].isName
	})
	for ident, te := range rs.types {
		if ident.isName {
			nameSet[ident.name] = struct{}{}
		}
		for _, key := range te.orderedKeys[:te.namesCount] {
			nameSet[key.name] = struct{}{}
		}
	}
	names := slices.Sorted(maps.Keys(nameSet))
	offset := rs.dirSize()
	for _, n := range names {
		s.nameOffset[n] = offset
		u := utf16.Encode([]rune(n))
		s.namesData = append(s.namesData, uint16(len(u)))
		s.namesData = append(s.namesData, u...)
		offset += (len(u) + 1) * 2
	}
	sz := len(s.namesData) * 2
	s.namesData = append(s.namesData, make([]uint16, (alignData(sz)-sz)/2)...)
	return s
}

func (rs *resourceSet) writeTypeDir(w io.Writer, s *rsrcState) error {
	if err := writeDirectoryTable(w, s.namesCount, len(s.orderedKeys)-s.namesCount); err != nil {
		return err
	}
	s.offset += sizeOfDirTable + len(s.orderedKeys)*sizeOfDirEntry
	for _, ident := range s.orderedKeys[:s.namesCount] {
		if err := writeDirectoryEntry(w, s.nameOffset[ident.name], s.offset, true, true); err != nil {
			return err
		}
		s.offset += rs.types[ident].size()
	}
	for _, ident := range s.orderedKeys[s.namesCount:] {
		if err := writeDirectoryEntry(w, int(ident.id), s.offset, false, true); err != nil {
			return err
		}
		s.offset += rs.types[ident].size()
	}
	return nil
}

func (rs *resourceSet) writeResDirs(w io.Writer, s *rsrcState) error {
	for _, tid := range s.orderedKeys {
		if err := rs.types[tid].write(w, s); err != nil {
			return err
		}
	}
	return nil
}

func (rs *resourceSet) writeLangDirs(w io.Writer, s *rsrcState) error {
	for _, tid := range s.orderedKeys {
		te := rs.types[tid]
		for _, ident := range te.orderedKeys {
			if err := te.resources[ident].write(w, s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rs *resourceSet) writeDataIndex(w io.Writer, s *rsrcState) error {
	for _, tid := range s.orderedKeys {
		te := rs.types[tid]
		for _, rid := range te.orderedKeys {
			re := te.resources[rid]
			for _, lcid := range re.orderedKeys {
				if err := re.data[lcid].writeIndex(w, s); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (rs *resourceSet) writeData(w io.Writer, s *rsrcState) error {
	for _, tid := range s.orderedKeys {
		te := rs.types[tid]
		for _, rid := range te.orderedKeys {
			re := te.resources[rid]
			for _, lcid := range re.orderedKeys {
				if err := re.data[lcid].writeData(w); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (rs *resourceSet) fullSize() int {
	s := rs.prepare()
	sz := rs.dirSize() + len(s.namesData)*2
	for _, te := range rs.types {
		for _, re := range te.resources {
			for _, de := range re.data {
				sz += de.paddedDataSize()
			}
		}
	}
	return sz
}

func (rs *resourceSet) dirSize() int {
	sz := sizeOfDirTable + len(rs.types)*sizeOfDirEntry
	for _, te := range rs.types {
		sz += te.size()
		for _, re := range te.resources {
			sz += re.size() + len(re.data)*sizeOfDataEntry
		}
	}
	return sz
}

func (rs *resourceSet) numDataEntries() int {
	var n int
	for _, te := range rs.types {
		for _, re := range te.resources {
			n += len(re.data)
		}
	}
	return n
}

type typeEntry struct {
	resources   map[identifier]*resourceEntry
	orderedKeys []identifier
	namesCount  int
}

func (te *typeEntry) size() int {
	return sizeOfDirTable + len(te.resources)*sizeOfDirEntry
}

func (te *typeEntry) order() {
	if te.orderedKeys != nil {
		return
	}
	te.orderedKeys = make([]identifier, 0, len(te.resources))
	for ident, re := range te.resources {
		te.orderedKeys = append(te.orderedKeys, ident)
		re.order()
	}
	sort.Slice(te.orderedKeys, func(i, j int) bool {
		return te.orderedKeys[i].lessThan(te.orderedKeys[j])
	})
	te.namesCount = sort.Search(len(te.orderedKeys), func(i int) bool {
		return !te.orderedKeys[i].isName
	})
}

func (te *typeEntry) write(w io.Writer, s *rsrcState) error {
	if err := writeDirectoryTable(w, te.namesCount, len(te.orderedKeys)-te.namesCount); err != nil {
		return err
	}
	for _, ident := range te.orderedKeys[:te.namesCount] {
		if err := writeDirectoryEntry(w, s.nameOffset[ident.name], s.offset, true, true); err != nil {
			return err
		}
		s.offset += te.resources[ident].size()
	}
	for _, ident := range te.orderedKeys[te.namesCount:] {
		if err := writeDirectoryEntry(w, int(ident.id), s.offset, false, true); err != nil {
			return err
		}
		s.offset += te.resources[ident].size()
	}
	return nil
}

type resourceEntry struct {
	data        map[uint16]*dataEntry
	orderedKeys []uint16
}

func (re *resourceEntry) size() int {
	return sizeOfDirTable + len(re.data)*sizeOfDirEntry
}

func (re *resourceEntry) order() {
	if re.orderedKeys != nil {
		return
	}
	re.orderedKeys = slices.Sorted(maps.Keys(re.data))
}

func (re *resourceEntry) write(w io.Writer, s *rsrcState) error {
	if err := writeDirectoryTable(w, 0, len(re.data)); err != nil {
		return err
	}
	for _, lcid := range re.orderedKeys {
		s.relocAddr = append(s.relocAddr, s.offset)
		if err := writeDirectoryEntry(w, int(lcid), s.offset, false, false); err != nil {
			return err
		}
		s.offset += sizeOfDataEntry
	}
	return nil
}

type dataEntry struct {
	data []byte
}

func alignData(offset int) int {
	return (offset + dataAlignment - 1) &^ (dataAlignment - 1)
}

func (de *dataEntry) paddedDataSize() int {
	return alignData(len(de.data))
}

func (de *dataEntry) writeIndex(w io.Writer, s *rsrcState) error {
	if err := writeDataEntry(w, s.offset, len(de.data)); err != nil {
		return err
	}
	s.offset += de.paddedDataSize()
	return nil
}

func (de *dataEntry) writeData(w io.Writer) error {
	n, err := w.Write(de.data)
	if err != nil {
		return errs.Wrap(err)
	}
	var pad [dataAlignment]byte
	_, err = w.Write(pad[:de.paddedDataSize()-n])
	return errs.Wrap(err)
}

type resourceDirectoryTable struct {
	Characteristics     uint32
	TimeDateStamp       uint32
	MajorVersion        uint16
	MinorVersion        uint16
	NumberOfNameEntries uint16
	NumberOfIDEntries   uint16
}

func writeDirectoryTable(w io.Writer, numNameEntries, numIDEntries int) error {
	return errs.Wrap(binary.Write(w, binary.LittleEndian, resourceDirectoryTable{
		NumberOfNameEntries: uint16(numNameEntries),
		NumberOfIDEntries:   uint16(numIDEntries),
	}))
}

type resourceDirectoryEntry struct {
	ID     uint32
	Offset uint32
}

func writeDirectoryEntry(w io.Writer, id, offset int, isName, isSubDir bool) error {
	e := resourceDirectoryEntry{ID: uint32(id), Offset: uint32(offset)}
	if isSubDir {
		e.Offset |= subDirFlag
	}
	if isName {
		e.ID |= nameFlag
	}
	return errs.Wrap(binary.Write(w, binary.LittleEndian, &e))
}

type resourceDataEntry struct {
	DataRVA  uint32
	Size     uint32
	Codepage uint32
	Reserved uint32
}

func writeDataEntry(w io.Writer, offset, dataSize int) error {
	return errs.Wrap(binary.Write(w, binary.LittleEndian, resourceDataEntry{
		DataRVA: uint32(offset),
		Size:    uint32(dataSize),
	}))
}

// winIcon describes a Windows icon as a set of images.
//
// https://docs.microsoft.com/en-us/previous-versions/ms997538
type winIcon struct {
	images []iconImage
}

type iconImage struct {
	image []byte
	info  iconInfo
}

type iconInfo struct {
	Width      uint8 // 0 means 256
	Height     uint8 // 0 means 256
	ColorCount uint8
	Reserved   uint8
	Planes     uint16
	BitCount   uint16
	BytesInRes uint32
}

type iconDirHeader struct {
	Reserved uint16
	Type     uint16
	Count    uint16
}

type iconResDirEntry struct {
	iconInfo
	ID uint16
}

func (rs *resourceSet) addWindowsIcons(appIconPath string, files []*FileData) error {
	if err := rs.addWindowIcon(appIconPath, "APP"); err != nil {
		return err
	}
	for _, fi := range files {
		if fi.Rank != "Owner" {
			continue
		}
		if err := rs.addWindowIcon(fi.Icon, fi.Extensions[0]); err != nil {
			return err
		}
	}
	return nil
}

func (rs *resourceSet) addWindowIcon(path, ext string) error {
	img, err := loadPNG(path)
	if err != nil {
		return err
	}
	ic := &winIcon{}
	for _, size := range []int{256, 64, 48, 32, 16} {
		if err = ic.addImage(resizeImage(img, size)); err != nil {
			return err
		}
	}
	return rs.setIcon(idName(ext), ic)
}

func loadPNG(path string) (img image.Image, err error) {
	var f *os.File
	if f, err = os.Open(path); err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errs.Wrap(closeErr)
		}
	}()
	img, err = png.Decode(f)
	return img, errs.Wrap(err)
}

func (ic *winIcon) addImage(img image.Image) error {
	bounds := img.Bounds()
	if bounds.Empty() {
		return errs.New("invalid image dimensions")
	}
	if bounds.Dx() > 256 || bounds.Dy() > 256 {
		return errs.New("image too big (max 256x256)")
	}
	img = imageInSquareNRGBA(img)
	bounds = img.Bounds()
	buf := &bytes.Buffer{}
	if err := png.Encode(buf, img); err != nil {
		return errs.Wrap(err)
	}
	ic.images = append(ic.images, iconImage{
		info: iconInfo{
			Width:      uint8(bounds.Dx()),
			Height:     uint8(bounds.Dy()),
			Planes:     1,
			BitCount:   32,
			BytesInRes: uint32(buf.Len()),
		},
		image: buf.Bytes(),
	})
	return nil
}

func (ic *winIcon) order() {
	sort.SliceStable(ic.images, func(i, j int) bool {
		a, b := &ic.images[i].info, &ic.images[j].info
		wa := int(a.Width)
		if wa == 0 {
			wa = 256
		}
		wb := int(b.Width)
		if wb == 0 {
			wb = 256
		}
		return a.BitCount > b.BitCount || (a.BitCount == b.BitCount && wa > wb)
	})
}

func resizeImage(img image.Image, size int) image.Image {
	b := img.Bounds()
	w, h := size, size
	switch {
	case b.Dx() < b.Dy():
		w = (size*b.Dx() + b.Dy()/2) / b.Dy()
	case b.Dx() > b.Dy():
		h = (size*b.Dy() + b.Dx()/2) / b.Dx()
	}
	dst := image.NewNRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

func imageInSquareNRGBA(img image.Image) image.Image {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w == h && img.ColorModel() == color.NRGBAModel {
		return img
	}
	length := max(w, h)
	offset := image.Point{X: -img.Bounds().Min.X, Y: -img.Bounds().Min.Y}
	offset.X -= (w - length) / 2
	offset.Y -= (h - length) / 2
	square := image.NewNRGBA(image.Rectangle{Max: image.Point{X: length, Y: length}})
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			square.Set(x+offset.X, y+offset.Y, img.At(x, y))
		}
	}
	return square
}

func makeManifest(description string) []byte {
	t := template.Must(template.New("manifest").Parse(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <description>{{. | html}}</description>

  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>

  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">permonitorv2,system</dpiAwareness>
      <highResolutionScrollingAware xmlns="http://schemas.microsoft.com/SMI/2013/WindowsSettings">true</highResolutionScrollingAware>
      <longPathAware xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">true</longPathAware>
    </windowsSettings>
  </application>

  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>

</assembly>
`))
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, description); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

const (
	codePageUTF16LE  = 1200
	sizeOfNodeHeader = 6

	vsVersionInfo  = "VS_VERSION_INFO"
	stringFileInfo = "StringFileInfo"
	varFileInfo    = "VarFileInfo"
	translation    = "Translation"
)

// versionInfo holds the parts of the version info that we need to fill in the manifest
//
// https://docs.microsoft.com/en-us/windows/win32/menurc/vs-versioninfo
type versionInfo struct {
	productName     string
	fullVersion     string
	shortVersion    string
	fileName        string
	companyName     string
	fileDescription string
	copyright       string
	trademarks      string
}

func (vi *versionInfo) bytes() []byte {
	buf := &bytes.Buffer{}
	writeAligned(buf, vi.fixedFileInfo())
	writeAligned(buf, vi.stringFileInfoBytes())
	writeAligned(buf, varFileInfoBytes())
	return nodeBytes(false, vsVersionInfo, buf.Bytes(), sizeOfFixedFileInfo)
}

// vsFixedFileInfo
//
// https://docs.microsoft.com/en-us/windows/win32/api/verrsrc/ns-verrsrc-vs_fixedfileinfo
type vsFixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

const (
	sizeOfFixedFileInfo    = 52
	fixedFileInfoSignature = 0xFEEF04BD
	fixedFileInfoVersion   = 0x10000
	vsFFMask               = 0x3F
	vosNTWindows32         = 0x040004
	vftApp                 = 1
)

func (vi *versionInfo) fixedFileInfo() vsFixedFileInfo {
	v := versionStringToArray(vi.fullVersion)
	return vsFixedFileInfo{
		Signature:        fixedFileInfoSignature,
		StrucVersion:     fixedFileInfoVersion,
		FileVersionMS:    uint32(v[0])<<16 | uint32(v[1]),
		FileVersionLS:    uint32(v[2])<<16 | uint32(v[3]),
		ProductVersionMS: uint32(v[0])<<16 | uint32(v[1]),
		ProductVersionLS: uint32(v[2])<<16 | uint32(v[3]),
		FileFlagsMask:    vsFFMask,
		FileOS:           vosNTWindows32,
		FileType:         vftApp,
	}
}

type nodeHeader struct {
	Length      uint16
	ValueLength uint16
	Type        uint16
}

func (vi *versionInfo) stringFileInfoBytes() []byte {
	buf := &bytes.Buffer{}
	// The keys must be written in ascending order; this list is already sorted.
	for _, kv := range [...]struct{ key, value string }{
		{"CompanyName", vi.companyName},
		{"FileDescription", vi.fileDescription},
		{"FileVersion", vi.shortVersion},
		{"InternalName", vi.fileName},
		{"LegalCopyright", vi.copyright},
		{"LegalTrademarks", vi.trademarks},
		{"OriginalFilename", vi.fileName},
		{"ProductName", vi.productName},
		{"ProductVersion", vi.shortVersion},
	} {
		writeAligned(buf, stringBytes(kv.key, kv.value))
	}
	table := nodeBytes(true, fmt.Sprintf("%04x%04x", lcidDefault, codePageUTF16LE), buf.Bytes(), 0)
	return nodeBytes(true, stringFileInfo, table, 0)
}

func stringBytes(key, value string) []byte {
	wValue := utf16.Encode([]rune(value + "\x00"))
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, wValue)
	return nodeBytes(true, key, buf.Bytes(), len(wValue))
}

func varFileInfoBytes() []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.LittleEndian, uint32(codePageUTF16LE)<<16|uint32(lcidDefault))
	return nodeBytes(true, varFileInfo, nodeBytes(false, translation, buf.Bytes(), buf.Len()), 0)
}

func nodeBytes(text bool, key string, value []byte, valueLength int) []byte {
	wKey := utf16.Encode([]rune(key + "\x00"))
	hdr := nodeHeader{Length: uint16(sizeOfNodeHeader + len(wKey)*2 + len(value)), ValueLength: uint16(valueLength)}
	if len(wKey)&1 == 0 {
		hdr.Length += 2
	}
	if text {
		hdr.Type = 1
	}
	buf := bytes.NewBuffer(make([]byte, 0, hdr.Length))
	binary.Write(buf, binary.LittleEndian, hdr)
	binary.Write(buf, binary.LittleEndian, wKey)
	writeAligned(buf, value)
	return buf.Bytes()
}

func writeAligned(buf *bytes.Buffer, data any) {
	var pad [4]byte
	s := buf.Len()
	binary.Write(buf, binary.LittleEndian, pad[:(s+3)&^3-s])
	binary.Write(buf, binary.LittleEndian, data)
}

func versionStringToArray(v string) [4]uint16 {
	var (
		part int
		ver  [4]uint16
		i    int
	)
	for i = range v {
		if v[i] >= '0' && v[i] <= '9' {
			break
		}
	}
	v = v[i:]
	for i = range v {
		switch {
		case v[i] >= '0' && v[i] <= '9':
			ver[part] = ver[part]*10 + uint16(v[i]-'0')
		case v[i] == '.' && part < 3:
			part++
		default:
			return ver
		}
	}
	return ver
}
