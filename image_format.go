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
	"net/url"
	"path"
	"strings"
)

// InvalidImageFormatStr is returned for as an error indicator for some methods on EncodedImageFormat.
const InvalidImageFormatStr = "\x00invalid"

// EncodedImageFormat holds the type of encoding an image was stored with.
type EncodedImageFormat uint8

// Possible values for EncodedImageFormat.
const (
	BMP EncodedImageFormat = iota
	GIF
	ICO
	JPEG
	PNG
	WBMP
	WEBP
	/*
		The following formats, though supported in theory, fail to work with the cskia builds I've done.

		PKM
		KTX
		ASTC
		DNG
		HEIF
	*/
	knownEncodedImageFormatCount
	UnknownEncodedImageFormat EncodedImageFormat = 255
)

type imageFormatInfo struct {
	Extensions []string
	MimeTypes  []string
	UTI        string
	CanWrite   bool
}

var knownImageFormats = []*imageFormatInfo{
	{
		Extensions: []string{".bmp", ".dib"},
		MimeTypes:  []string{"image/bmp", "image/x-bmp"},
		UTI:        "com.microsoft.bmp",
	},
	{
		Extensions: []string{".gif"},
		MimeTypes:  []string{"image/gif"},
		UTI:        "com.compuserve.gif",
	},
	{
		Extensions: []string{".ico"},
		MimeTypes:  []string{"image/x-icon", "image/vnd.microsoft.icon"},
		UTI:        "com.microsoft.ico",
	},
	{
		Extensions: []string{".jpg", ".jpeg", ".jpe", ".jif", ".jfif", ".jfi"},
		MimeTypes:  []string{"image/jpeg"},
		UTI:        "public.jpeg",
		CanWrite:   true,
	},
	{
		Extensions: []string{".png"},
		MimeTypes:  []string{"image/png"},
		UTI:        "public.png",
		CanWrite:   true,
	},
	{
		Extensions: []string{".wbmp"},
		MimeTypes:  []string{"image/vnd.wap.wbmp"},
		UTI:        "com.adobe.wbmp",
	},
	{
		Extensions: []string{".webp"},
		MimeTypes:  []string{"image/webp"},
		UTI:        "org.webmproject.webp",
		CanWrite:   true,
	},
}

var (
	// KnownImageFormatFormats holds the list of known image file formats.
	KnownImageFormatFormats = make([]EncodedImageFormat, len(knownImageFormats))
	// KnownImageFormatExtensions holds the list of known image file format extensions.
	KnownImageFormatExtensions []string
	mimeTypeToImageFormat      = make(map[string]EncodedImageFormat)
	extensionToImageFormat     = make(map[string]EncodedImageFormat)
)

func init() {
	for i, one := range knownImageFormats {
		KnownImageFormatFormats[i] = EncodedImageFormat(i)
		for _, ext := range one.Extensions {
			extensionToImageFormat[ext] = EncodedImageFormat(i)
			KnownImageFormatExtensions = append(KnownImageFormatExtensions, ext)
		}
		for _, mimeType := range one.MimeTypes {
			mimeTypeToImageFormat[mimeType] = EncodedImageFormat(i)
		}
	}
}

func (e EncodedImageFormat) String() string {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].Extensions[0][1:]
	}
	return "unknown format"
}

// CanRead returns true if the format can be read.
func (e EncodedImageFormat) CanRead() bool {
	return e < knownEncodedImageFormatCount
}

// CanWrite returns true if the format can be written.
func (e EncodedImageFormat) CanWrite() bool {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].CanWrite
	}
	return false
}

// Extensions returns the list of valid extensions for the format. An unknown / invalid format will return nil.
func (e EncodedImageFormat) Extensions() []string {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].Extensions
	}
	return nil
}

// Extension returns the primary extension for the format. An unknown / invalid format will return InvalidImageFormatStr.
func (e EncodedImageFormat) Extension() string {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].Extensions[0]
	}
	return InvalidImageFormatStr
}

// MimeTypes returns the list of valid mime types for the format. An unknown / invalid format will return nil.
func (e EncodedImageFormat) MimeTypes() []string {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].MimeTypes
	}
	return nil
}

// MimeType returns the primary mime type for the format. An unknown / invalid format will return InvalidImageFormatStr.
func (e EncodedImageFormat) MimeType() string {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].MimeTypes[0]
	}
	return InvalidImageFormatStr
}

// UTI returns the uniform type identifier for the format. An unknown / invalid format will return
// InvalidImageFormatStr.
func (e EncodedImageFormat) UTI() string {
	if e < knownEncodedImageFormatCount {
		return knownImageFormats[e].UTI
	}
	return InvalidImageFormatStr
}

// EncodedImageFormatForPath returns the EncodedImageFormat associated with the extension of the given path.
func EncodedImageFormatForPath(p string) EncodedImageFormat {
	return EncodedImageFormatForExtension(path.Ext(p))
}

// EncodedImageFormatForExtension returns the EncodedImageFormat associated with the extension.
func EncodedImageFormatForExtension(ext string) EncodedImageFormat {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if e, ok := extensionToImageFormat[strings.ToLower(ext)]; ok {
		return e
	}
	return UnknownEncodedImageFormat
}

// EncodedImageFormatForMimeType returns the EncodedImageFormat associated with the mime type.
func EncodedImageFormatForMimeType(mimeType string) EncodedImageFormat {
	if e, ok := mimeTypeToImageFormat[strings.ToLower(mimeType)]; ok {
		return e
	}
	return UnknownEncodedImageFormat
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
