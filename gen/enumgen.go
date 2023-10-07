/*
 * Copyright ©2021-2023 by Richard A. Wilkes. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, version 2.0. If a copy of the MPL was not distributed with
 * this file, You can obtain one at http://mozilla.org/MPL/2.0/.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as
 * defined by the Mozilla Public License, version 2.0.
 */

package main

//go:generate go run enumgen.go

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"

	"github.com/richardwilkes/toolbox/fatal"
	"github.com/richardwilkes/toolbox/txt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	rootDir   = ".."
	genSuffix = "_gen.go"
	enumTmpl  = "enum.go.tmpl"
)

type enumValue struct {
	Name          string
	Key           string
	OldKeys       []string
	String        string
	Alt           string
	Comment       string
	NoLocalize    bool
	EmptyStringOK bool
	ForceUpper    bool
}

type enumInfo struct {
	Pkg      string
	Name     string
	Desc     string
	BaseType string
	Values   []enumValue
}

func main() {
	removeExistingGenFiles()
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/align",
		Name:     "align",
		Desc:     "specifies how to align an object within its available space",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "start"},
			{Key: "middle"},
			{Key: "end"},
			{Key: "fill"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/behavior",
		Name:     "behavior",
		Desc:     "controls how auto-sizing of the scroll content's preferred size is handled",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "unmodified"},
			{Key: "fill", Comment: "If the content is smaller than the available space, expand it"},
			{Key: "follow", Comment: "Fix the content to the view size"},
			{Key: "hinted_fill", Comment: "Uses hints to try and fix the content to the view size, but if the resulting content is smaller than the available space, expands it"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/check",
		Name:     "check",
		Desc:     "represents the current state of something like a check box or mark",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "off"},
			{Key: "on"},
			{Key: "mixed"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/filtermode",
		Name:     "filtermode",
		Desc:     "holds the type of sampling to be done",
		BaseType: "int32",
		Values: []enumValue{
			{Key: "nearest", Comment: "Single sample point (nearest neighbor)"},
			{Key: "linear", Comment: "Interpolate between 2x2 sample points (bilinear interpolation)"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/imgfmt",
		Name:     "imgfmt",
		Desc:     "holds the type of encoding an image was stored with",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "unknown"},
			{Key: "bmp", NoLocalize: true, ForceUpper: true},
			{Key: "gif", NoLocalize: true, ForceUpper: true},
			{Key: "ico", NoLocalize: true, ForceUpper: true},
			{Key: "jpeg", NoLocalize: true, ForceUpper: true},
			{Key: "png", NoLocalize: true, ForceUpper: true},
			{Key: "wbmp", NoLocalize: true, ForceUpper: true},
			{Key: "webp", NoLocalize: true, ForceUpper: true},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/invertstyle",
		Name:     "invertstyle",
		Desc:     "holds the type of image inversion",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "none"},
			{Key: "brightness"},
			{Key: "lightness"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/mipmapmode",
		Name:     "mipmapmode",
		Desc:     "holds the type of mipmapping to be done",
		BaseType: "int32",
		Values: []enumValue{
			{Key: "none", Comment: "Ignore mipmap levels, sample from the 'base'"},
			{Key: "nearest", Comment: "Sample from the nearest level"},
			{Key: "linear", Comment: "Interpolate between the two nearest levels"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/side",
		Name:     "side",
		Desc:     "specifies which side an object should be on",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "top"},
			{Key: "left"},
			{Key: "bottom"},
			{Key: "right"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/thememode",
		Name:     "thememode",
		Desc:     "holds the theme display mode",
		BaseType: "byte",
		Values: []enumValue{
			{Key: "auto", String: "Automatic"},
			{Key: "dark"},
			{Key: "light"},
		},
	})
}

func removeExistingGenFiles() {
	root, err := filepath.Abs(rootDir)
	fatal.IfErr(err)
	fatal.IfErr(filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		name := info.Name()
		if info.IsDir() {
			if name == ".git" {
				return filepath.SkipDir
			}
		} else {
			if strings.HasSuffix(name, genSuffix) {
				fatal.IfErr(os.Remove(path))
			}
		}
		return nil
	}))
}

func processSourceTemplate(tmplName string, info *enumInfo) {
	tmpl, err := template.New(tmplName).Funcs(template.FuncMap{
		"add":          add,
		"emptyIfTrue":  emptyIfTrue,
		"fileLeaf":     filepath.Base,
		"firstToLower": txt.FirstToLower,
		"join":         join,
		"last":         last,
		"toCamelCase":  txt.ToCamelCase,
		"toIdentifier": toIdentifier,
		"wrapComment":  wrapComment,
	}).ParseFiles(tmplName)
	fatal.IfErr(err)
	var buffer bytes.Buffer
	writeGeneratedFromComment(&buffer, tmplName)
	fatal.IfErr(tmpl.Execute(&buffer, info))
	var data []byte
	if data, err = format.Source(buffer.Bytes()); err != nil {
		fmt.Println("unable to format source file: " + filepath.Join(info.Pkg, info.Name+genSuffix))
		data = buffer.Bytes()
	}
	dir := filepath.Join(rootDir, info.Pkg)
	fatal.IfErr(os.MkdirAll(dir, 0o750))
	fatal.IfErr(os.WriteFile(filepath.Join(dir, info.Name+genSuffix), data, 0o640))
}

func writeGeneratedFromComment(w io.Writer, tmplName string) {
	_, err := fmt.Fprintf(w, "// Code generated from \"%s\" - DO NOT EDIT.\n\n", tmplName)
	fatal.IfErr(err)
}

func add(a, b int) int {
	return a + b
}

func join(values []string) string {
	var buffer strings.Builder
	for i, one := range values {
		if i != 0 {
			buffer.WriteString(", ")
		}
		fmt.Fprintf(&buffer, "%q", one)
	}
	return buffer.String()
}

func (e *enumInfo) LocalType() string {
	return txt.FirstToLower(toIdentifier(e.Name)) + "Data"
}

func (e *enumInfo) IDFor(v enumValue) string {
	id := v.Name
	if id == "" {
		id = toIdentifier(v.Key)
	}
	if v.ForceUpper {
		id = strings.ToUpper(id)
	}
	return id
}

func (e *enumInfo) HasAlt() bool {
	for _, one := range e.Values {
		if one.Alt != "" {
			return true
		}
	}
	return false
}

func (e *enumInfo) HasOldKeys() bool {
	for _, one := range e.Values {
		if len(one.OldKeys) != 0 {
			return true
		}
	}
	return false
}

func (e *enumInfo) NeedI18N() bool {
	for _, one := range e.Values {
		if !one.NoLocalize || one.Alt != "" {
			return true
		}
	}
	return false
}

func (e *enumValue) StringValue() string {
	if e.String == "" && !e.EmptyStringOK {
		key := strings.ReplaceAll(e.Key, "_", " ")
		if e.ForceUpper {
			return strings.ToUpper(key)
		}
		return cases.Title(language.AmericanEnglish).String(key)
	}
	return e.String
}

func last(in []enumValue) enumValue {
	return in[len(in)-1]
}

func emptyIfTrue(str string, test bool) string {
	if test {
		return ""
	}
	return str
}

func toIdentifier(in string) string {
	var buffer strings.Builder
	useUpper := true
	for i, ch := range in {
		isUpper := ch >= 'A' && ch <= 'Z'
		isLower := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		isAlpha := isUpper || isLower
		if i == 0 && !isAlpha {
			if !isDigit {
				continue
			}
			buffer.WriteString("_")
		}
		if isAlpha {
			if useUpper {
				buffer.WriteRune(unicode.ToUpper(ch))
			} else {
				buffer.WriteRune(unicode.ToLower(ch))
			}
			useUpper = false
		} else {
			if isDigit {
				buffer.WriteRune(ch)
			}
			useUpper = true
		}
	}
	return buffer.String()
}

func wrapComment(in string, cols int) string {
	return txt.Wrap("// ", in, cols)
}
