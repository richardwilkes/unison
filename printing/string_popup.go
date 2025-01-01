// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package printing

import (
	"strings"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Language is the language that will be used when performing string manipulations for display.
var Language = language.AmericanEnglish // TODO: Use the user's default

type stringPopup[T ~string] struct {
	colorMode *unison.PopupMenu[T]
}

func createCapStringPopup[T ~string](parent *unison.Panel, label string) stringPopup[T] {
	p := stringPopup[T]{colorMode: unison.NewPopupMenu[T]()}
	p.colorMode.SetLayoutData(&unison.FlexLayoutData{VAlign: align.Middle})
	parent.AddChild(createLabel(label))
	parent.AddChild(p.colorMode)
	return p
}

func (p stringPopup[T]) rebuild(supported func() []string, current, def func() string) {
	p.colorMode.RemoveAllItems()
	list := supported()
	if len(list) != 0 {
		c := current()
		if c == "" {
			c = def()
		}
		var sel string
		for _, s := range list {
			if sel == "" {
				sel = s
			}
			p.colorMode.AddItem(T(s))
			if s == c {
				sel = c
			}
		}
		p.colorMode.Select(T(sel))
	} else {
		p.colorMode.AddItem(T(i18n.Text("Not Applicable")))
		p.colorMode.SelectIndex(0)
	}
	p.colorMode.MarkForLayoutAndRedraw()
	if parent := p.colorMode.Parent(); parent != nil {
		parent.NeedsLayout = true
	}
}

func (p stringPopup[T]) setEnabled(enabled bool, supported func() []string) {
	p.colorMode.SetEnabled(enabled && len(supported()) != 0)
}

func (p stringPopup[T]) apply(supported func() []string, set func(string)) {
	if len(supported()) != 0 {
		colorMode, _ := p.colorMode.Selected()
		set(string(colorMode))
	} else {
		set("")
	}
}

type capString string

func (s capString) String() string {
	return cases.Title(Language).String(strings.ReplaceAll(strings.ReplaceAll(string(s), "_", " "), "-", " "))
}

type mediaString string

func (s mediaString) String() string {
	parts := make([]string, 0, 3)
	txt := strings.ReplaceAll(strings.ReplaceAll(string(s), "_", " "), "-", " ")
	for _, prefix := range []string{"iso ", "jis ", "na "} {
		if strings.HasPrefix(txt, prefix) {
			parts = append(parts, strings.ToUpper(prefix))
			txt = strings.TrimPrefix(txt, prefix)
			break
		}
	}
	if i := strings.LastIndex(txt, " "); i != -1 {
		parts = append(parts, cases.Title(Language).String(txt[:i]), "("+txt[i+1:]+")")
	} else {
		parts = append(parts, cases.Title(Language).String(txt))
	}
	return strings.Join(parts, " ")
}
