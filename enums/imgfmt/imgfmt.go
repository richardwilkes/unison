// Copyright ©2021-2023 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package imgfmt

import (
	"net/url"
	"path"
	"strings"
)

// CanRead returns true if the format can be read.
func (e Enum) CanRead() bool {
	return e.EnsureValid() != Unknown
}

// CanWrite returns true if the format can be written.
func (e Enum) CanWrite() bool {
	switch e {
	case JPEG, PNG, WEBP:
		return true
	default:
		return false
	}
}

// Extensions returns the list of valid file extensions for the format. An unknown format will return nil.
func (e Enum) Extensions() []string {
	switch e {
	case BMP:
		return []string{".bmp", ".dib"}
	case GIF:
		return []string{".gif"}
	case ICO:
		return []string{".ico"}
	case JPEG:
		return []string{".jpg", ".jpeg", ".jpe", ".jif", ".jfif", ".jfi"}
	case PNG:
		return []string{".png"}
	case WBMP:
		return []string{".wbmp"}
	case WEBP:
		return []string{".webp"}
	default:
		return nil
	}
}

// Extension returns the primary extension for the format. An unknown format will return an empty string.
func (e Enum) Extension() string {
	ext := e.Extensions()
	if ext == nil {
		return ""
	}
	return ext[0]
}

// MimeTypes returns the list of valid mime types for the format. An unknown format will return nil.
func (e Enum) MimeTypes() []string {
	switch e {
	case BMP:
		return []string{"image/bmp", "image/x-bmp"}
	case GIF:
		return []string{"image/gif"}
	case ICO:
		return []string{"image/x-icon", "image/vnd.microsoft.icon"}
	case JPEG:
		return []string{"image/jpeg"}
	case PNG:
		return []string{"image/png"}
	case WBMP:
		return []string{"image/vnd.wap.wbmp"}
	case WEBP:
		return []string{"image/webp"}
	default:
		return nil
	}
}

// MimeType returns the primary mime type for the format. An unknown format will return an empty string.
func (e Enum) MimeType() string {
	types := e.MimeTypes()
	if types == nil {
		return ""
	}
	return types[0]
}

// UTI returns the uniform type identifier for the format. An unknown format will return an empty string.
func (e Enum) UTI() string {
	switch e {
	case BMP:
		return "com.microsoft.bmp"
	case GIF:
		return "com.compuserve.gif"
	case ICO:
		return "com.microsoft.ico"
	case JPEG:
		return "public.jpeg"
	case PNG:
		return "public.png"
	case WBMP:
		return "com.adobe.wbmp"
	case WEBP:
		return "org.webmproject.webp"
	default:
		return ""
	}
}

// AllReadableExtensions returns all file extensions that map to readable image formats.
func AllReadableExtensions() []string {
	all := make([]string, 0, 16)
	for _, e := range All {
		if e.CanRead() {
			all = append(all, e.Extensions()...)
		}
	}
	return all
}

// AllWritableExtensions returns all file extensions that map to writable image formats.
func AllWritableExtensions() []string {
	all := make([]string, 0, 16)
	for _, e := range All {
		if e.CanWrite() {
			all = append(all, e.Extensions()...)
		}
	}
	return all
}

// ForPath returns the image format for the given file path's extension.
func ForPath(p string) Enum {
	return ForExtension(path.Ext(p))
}

// ForExtension returns the image format for the given file extension.
func ForExtension(ext string) Enum {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	for _, e := range All {
		if e == Unknown {
			continue
		}
		for _, one := range e.Extensions() {
			if strings.EqualFold(ext, one) {
				return e
			}
		}
	}
	return Unknown
}

// ForMimeType returns the image format for the given mime type.
func ForMimeType(mimeType string) Enum {
	for _, e := range All {
		if e == Unknown {
			continue
		}
		for _, one := range e.MimeTypes() {
			if strings.EqualFold(mimeType, one) {
				return e
			}
		}
	}
	return Unknown
}

// ForUTI returns the image format for the given Universal Type Identifier.
func ForUTI(uti string) Enum {
	for _, e := range All {
		if e == Unknown {
			continue
		}
		if strings.EqualFold(uti, e.UTI()) {
			return e
		}
	}
	return Unknown
}

// Distill a file path or URL string into one that likely has an image we can read, or an empty string.
func Distill(filePathOrURL string) string {
	if u, err := url.Parse(filePathOrURL); err == nil {
		switch u.Scheme {
		case "file":
			if e := ForPath(filePathOrURL); e.CanRead() {
				return filePathOrURL
			}
			return ""
		case "http", "https":
			if e := ForPath(u.Path); e.CanRead() {
				return filePathOrURL
			}
			if alt, ok := u.Query()["imgurl"]; ok && len(alt) > 0 {
				return Distill(alt[0])
			}
			const revisionLatest = "/revision/latest"
			if strings.HasSuffix(u.Path, revisionLatest) {
				u.RawPath = ""
				u.Path = u.Path[:len(u.Path)-len(revisionLatest)]
				return Distill(u.String())
			}
			return ""
		default:
		}
	}
	// We may have been passed a raw file path
	if e := ForPath(filePathOrURL); e.CanRead() {
		return filePathOrURL
	}
	return ""
}
