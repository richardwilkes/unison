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
	Default       bool
	NoLocalize    bool
	EmptyStringOK bool
	ForceUpper    bool
}

type enumInfo struct {
	Pkg           string
	Name          string
	Desc          string
	baseType      string
	baseValue     string
	NonContiguous bool
	Values        []enumValue
}

func main() {
	removeExistingGenFiles()
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/align",
		Name: "align",
		Desc: "specifies how to align an object within its available space",
		Values: []enumValue{
			{Key: "start"},
			{Key: "middle"},
			{Key: "end"},
			{Key: "fill"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/arcsize",
		Name: "arcsize",
		Desc: "holds the relative size of an arc",
		Values: []enumValue{
			{Key: "small"},
			{Key: "large"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/behavior",
		Name: "behavior",
		Desc: "controls how auto-sizing of the scroll content's preferred size is handled",
		Values: []enumValue{
			{Key: "unmodified"},
			{Key: "fill", Comment: "If the content is smaller than the available space, expand it"},
			{Key: "follow", Comment: "Fix the content to the view size"},
			{Key: "hinted-fill", Comment: "Uses hints to try and fix the content to the view size, but if the resulting content is smaller than the available space, expands it"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/blendmode",
		Name: "blendmode",
		Desc: "holds the mode used for blending pixels",
		Values: []enumValue{
			{Key: "clear"},
			{Key: "src"},
			{Key: "dst"},
			{Key: "src-over"},
			{Key: "dst-over"},
			{Key: "src-in"},
			{Key: "dst-in"},
			{Key: "src-out"},
			{Key: "dst-out"},
			{Key: "src-atop"},
			{Key: "dst-atop"},
			{Key: "xor"},
			{Key: "plus"},
			{Key: "modulate"},
			{Key: "screen"},
			{Key: "overlay"},
			{Key: "darken"},
			{Key: "lighten"},
			{Key: "color-dodge"},
			{Key: "color-burn"},
			{Key: "hard-light"},
			{Key: "soft-light"},
			{Key: "difference"},
			{Key: "exclusion"},
			{Key: "multiply"},
			{Key: "hue"},
			{Key: "saturation"},
			{Key: "color"},
			{Key: "luminosity"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/blur",
		Name: "blur",
		Desc: "holds the type of blur to apply",
		Values: []enumValue{
			{Key: "normal"},
			{Key: "solid"},
			{Key: "outer"},
			{Key: "inner"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/check",
		Name: "check",
		Desc: "represents the current state of something like a check box or mark",
		Values: []enumValue{
			{Key: "off"},
			{Key: "on"},
			{Key: "mixed"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/colorchannel",
		Name: "colorchannel",
		Desc: "specifies a specific channel within an RGBA color",
		Values: []enumValue{
			{Key: "red"},
			{Key: "green"},
			{Key: "blue"},
			{Key: "alpha"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/direction",
		Name: "direction",
		Desc: "holds the direction of a path",
		Values: []enumValue{
			{Key: "clockwise"},
			{Key: "counter-clockwise"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:      "enums/filtermode",
		Name:     "filtermode",
		Desc:     "holds the type of sampling to be done",
		baseType: "int32",
		Values: []enumValue{
			{Key: "nearest", Comment: "Single sample point (nearest neighbor)"},
			{Key: "linear", Comment: "Interpolate between 2x2 sample points (bilinear interpolation)"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/imgfmt",
		Name: "imgfmt",
		Desc: "holds the type of encoding an image was stored with",
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
		Pkg:  "enums/invertstyle",
		Name: "invertstyle",
		Desc: "holds the type of image inversion",
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
		baseType: "int32",
		Values: []enumValue{
			{Key: "none", Comment: "Ignore mipmap levels, sample from the 'base'"},
			{Key: "nearest", Comment: "Sample from the nearest level"},
			{Key: "linear", Comment: "Interpolate between the two nearest levels"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/paintstyle",
		Name: "paintstyle",
		Desc: "holds the type of painting to do",
		Values: []enumValue{
			{Key: "fill"},
			{Key: "stroke"},
			{Key: "stroke-and-fill"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/patheffect",
		Name: "patheffect",
		Desc: "holds the 1D path effect",
		Values: []enumValue{
			{Key: "translate"},
			{Key: "rotate"},
			{Key: "morph"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/pathop",
		Name: "pathop",
		Desc: "holds the possible operations that can be performed on a pair of paths",
		Values: []enumValue{
			{Key: "difference"},
			{Key: "intersect"},
			{Key: "union"},
			{Key: "xor"},
			{Key: "reverse-difference"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/pointmode",
		Name: "pointmode",
		Desc: "controls how Canvas.DrawPoints() renders the points passed to it",
		Values: []enumValue{
			{Key: "points"},
			{Key: "lines"},
			{Key: "polygon"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/side",
		Name: "side",
		Desc: "specifies which side an object should be on",
		Values: []enumValue{
			{Key: "top"},
			{Key: "left"},
			{Key: "bottom"},
			{Key: "right"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/slant",
		Name: "slant",
		Desc: "holds the slant of a font",
		Values: []enumValue{
			{Key: "upright"},
			{Key: "italic"},
			{Key: "oblique"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:       "enums/spacing",
		Name:      "spacing",
		Desc:      "holds the text spacing of a font",
		baseValue: "iota + 1",
		Values: []enumValue{
			{Key: "ultra-condensed"},
			{Key: "extra-condensed"},
			{Key: "condensed"},
			{Key: "semi-condensed"},
			{Key: "standard", Default: true},
			{Key: "semi-expanded"},
			{Key: "expanded"},
			{Key: "extra-expanded"},
			{Key: "ultra-expanded"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/strokecap",
		Name: "strokecap",
		Desc: "holds the style for rendering the endpoint of a stroked line",
		Values: []enumValue{
			{Key: "butt"},
			{Key: "round"},
			{Key: "square"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/strokejoin",
		Name: "strokejoin",
		Desc: "holds the method for drawing the junction between connected line segments",
		Values: []enumValue{
			{Key: "miter"},
			{Key: "round"},
			{Key: "bevel"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/thememode",
		Name: "thememode",
		Desc: "holds the theme display mode",
		Values: []enumValue{
			{Key: "auto", String: "Automatic"},
			{Key: "dark"},
			{Key: "light"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/tilemode",
		Name: "tilemode",
		Desc: "holds the type of tiling to perform",
		Values: []enumValue{
			{Key: "clamp"},
			{Key: "repeat"},
			{Key: "mirror"},
			{Key: "decal"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/trimmode",
		Name: "trimmode",
		Desc: "holds the type of trim",
		Values: []enumValue{
			{Key: "normal"},
			{Key: "inverted"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:           "enums/weight",
		Name:          "weight",
		Desc:          "holds the wegith of a font",
		baseType:      "int32",
		baseValue:     "iota * 100",
		NonContiguous: true,
		Values: []enumValue{
			{Key: "invisible"},
			{Key: "thin"},
			{Key: "extra-light"},
			{Key: "light"},
			{Key: "regular", Default: true},
			{Key: "medium"},
			{Key: "semi-bold"},
			{Key: "bold"},
			{Key: "extra-bold"},
			{Key: "black"},
			{Key: "extra-black"},
		},
	})
	processSourceTemplate(enumTmpl, &enumInfo{
		Pkg:  "enums/filltype",
		Name: "filltype",
		Desc: "holds the type of fill operation to perform, which affects how overlapping contours interact with each other",
		Values: []enumValue{
			{Key: "winding"},
			{Key: "even-odd"},
			{Key: "inverse-winding"},
			{Key: "inverse-even-odd"},
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

func (e *enumInfo) BaseType() string {
	if e.baseType == "" {
		return "byte"
	}
	return e.baseType
}

func (e *enumInfo) BaseValue() string {
	if e.baseValue == "" {
		return "iota"
	}
	return e.baseValue
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

func (e *enumInfo) NeedLowerBoundsCheck() bool {
	return e.baseValue != "" || (e.baseType != "" && e.baseType != "byte" && e.baseType != "uint8" &&
		e.baseType != "uint16" && e.baseType != "uint32" && e.baseType != "uint64" && e.baseType != "uint")
}

func (e *enumInfo) First() enumValue {
	return e.Values[0]
}

func (e *enumInfo) Default() enumValue {
	for _, one := range e.Values {
		if one.Default {
			return one
		}
	}
	return e.Values[0]
}

func (e *enumInfo) Last() enumValue {
	return e.Values[len(e.Values)-1]
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
