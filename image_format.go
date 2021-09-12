// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
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
	"path"
	"strings"
)

// EncodedImageFormat holds the type of encoding an image was stored with.
type EncodedImageFormat byte

// Possible values for EncodedImageFormat.
const (
	BMP EncodedImageFormat = iota
	GIF
	ICO
	JPEG
	PNG
	WBMP
	WEBP
	PKM
	KTX
	ASTC
	DNG
	HEIF
	UnknownEncodedImageFormat EncodedImageFormat = 255
)

var imageFormatToExtensions = map[EncodedImageFormat][]string{
	BMP:  {".bmp"},
	GIF:  {".gif"},
	ICO:  {".ico"},
	JPEG: {".jpg", ".jpeg"},
	PNG:  {".png"},
	WBMP: {".wbmp"},
	WEBP: {".webp"},
	PKM:  {".pkm"},
	KTX:  {".ktx"},
	ASTC: {".astc"},
	DNG:  {".dng"},
	HEIF: {".heif", ".heic"},
}

var (
	// KnownImageFormatExtensions holds the list of known image file format extensions.
	KnownImageFormatExtensions []string
	extensionToImageFormat     = make(map[string]EncodedImageFormat)
)

func init() {
	for k, v := range imageFormatToExtensions {
		for _, one := range v {
			extensionToImageFormat[one] = k
			KnownImageFormatExtensions = append(KnownImageFormatExtensions, one)
		}
	}
}

func (e EncodedImageFormat) String() string {
	if s, ok := imageFormatToExtensions[e]; ok {
		return s[0][1:]
	}
	return "unknown format"
}

// CanRead returns true if the format can be read.
func (e EncodedImageFormat) CanRead() bool {
	return e <= HEIF
}

// CanWrite returns true if the format can be written.
func (e EncodedImageFormat) CanWrite() bool {
	return e == JPEG || e == PNG || e == WEBP
}

// Extensions returns the list of valid extensions for the format. An unknown / invalid format will return nil.
func (e EncodedImageFormat) Extensions() []string {
	return imageFormatToExtensions[e]
}

// Extension returns the primary extension for the format. An unknown / invalid format will return "\x00invalid".
func (e EncodedImageFormat) Extension() string {
	if s, ok := imageFormatToExtensions[e]; ok {
		return s[0]
	}
	return "\x00invalid"
}

// EncodedImageFormatForPath returns the EncodedImageFormat associated with the extension of the given path.
func EncodedImageFormatForPath(p string) EncodedImageFormat {
	e, ok := extensionToImageFormat[strings.ToLower(path.Ext(p))]
	if !ok {
		return UnknownEncodedImageFormat
	}
	return e
}

// DistillImageSpecFor distills a file path or URL string into one that likely has an image we can read, or an empty
// string.
func DistillImageSpecFor(filePathOrURL string) string {
	if u, err := url.Parse(filePathOrURL); err == nil {
		switch u.Scheme {
		case "file":
			if e := EncodedImageFormatForPath(filePathOrURL); e.CanRead() {
				return filePathOrURL
			}
			return ""
		case "http", "https":
			if e := EncodedImageFormatForPath(u.Path); e.CanRead() {
				return filePathOrURL
			}
			if alt, ok := u.Query()["imgurl"]; ok && len(alt) > 0 {
				return DistillImageSpecFor(alt[0])
			}
			const revisionLatest = "/revision/latest"
			if strings.HasSuffix(u.Path, revisionLatest) {
				u.RawPath = ""
				u.Path = u.Path[:len(u.Path)-len(revisionLatest)]
				return DistillImageSpecFor(u.String())
			}
			return ""
		default:
		}
	}
	// We may have been passed a raw file path... so try to open that
	if e := EncodedImageFormatForPath(filePathOrURL); e.CanRead() {
		return filePathOrURL
	}
	return ""
}
