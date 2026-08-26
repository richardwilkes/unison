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
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestAPIWrappersLiveInPlatformAPI checks the claim platform_api.go makes about itself: every api* function or method
// in the package is declared there, so that it really is the complete list and the only place deciding where each one
// goes. The files for every GOOS are parsed, since a build constraint is exactly what would let a stray one hide from
// a build on the machine running this.
func TestAPIWrappersLiveInPlatformAPI(t *testing.T) {
	c := check.New(t)
	entries, err := os.ReadDir(".")
	c.NoError(err)
	fset := token.NewFileSet()
	var strays []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") ||
			name == "platform_api.go" {
			continue
		}
		f, parseErr := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		c.NoError(parseErr)
		if f == nil {
			continue
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && isAPIWrapperName(fn.Name.Name) {
				strays = append(strays, fset.Position(fn.Pos()).String()+": "+fn.Name.Name)
			}
		}
	}
	c.Equal(0, len(strays), "every api* function must be declared in platform_api.go, but these are not: %v", strays)
}

// isAPIWrapperName reports whether name is of the form the platform wrappers use: "api" followed by a capitalized
// word, which leaves an ordinary identifier that merely begins with those letters alone.
func isAPIWrapperName(name string) bool {
	rest, ok := strings.CutPrefix(name, "api")
	return ok && rest != "" && unicode.IsUpper(rune(rest[0]))
}
