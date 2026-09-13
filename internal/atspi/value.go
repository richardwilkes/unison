// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// valueInterface returns the org.a11y.atspi.Value interface of a node that holds a number, such as a slider, a progress
// bar or a spin button. Text is the value as the widget itself would show it, which is how an assistive technology says
// "42%" rather than "0.42".
func (o *nodeObject) valueInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceValue,
		Properties: []*dbus.Property{
			{Name: "MinimumValue", Sig: "d", Get: func() (any, error) { return o.node.Min, nil }},
			{Name: "MaximumValue", Sig: "d", Get: func() (any, error) { return o.node.Max, nil }},
			{Name: "MinimumIncrement", Sig: "d", Get: func() (any, error) { return o.node.Step, nil }},
			{
				Name: "CurrentValue",
				Sig:  "d",
				Get:  func() (any, error) { return o.node.Number, nil },
				Set:  o.currentValueSetter(),
			},
			{Name: "Text", Sig: "s", Get: func() (any, error) { return o.node.Value, nil }},
		},
	}
}

// currentValueSetter returns the handler that takes a new value, or nil for a control whose value cannot be changed,
// such as a progress bar. The difference is visible before anything is written: a property with no setter is
// introspected as read-only rather than as readwrite, so a client that trusts what the object says about itself never
// attempts the write, instead of discovering only from the refusal that the value was never going to be taken.
func (o *nodeObject) currentValueSetter() func(any) error {
	if !o.node.Actions.Has(accessibility.SetValue) {
		return nil
	}
	return o.setCurrentValue
}

// setCurrentValue asks the widget to take a new numeric value. It returns as soon as the request has been handed to the
// user interface thread, so a client that reads the property back at once still sees the old value; the change is
// reported as an event when the next snapshot is published.
func (o *nodeObject) setCurrentValue(v any) error {
	value, ok := v.(float64)
	if !ok {
		return dbus.Errorf(dbus.InvalidArgs, "a double is required")
	}
	if !o.a.dispatch(accessibility.ActionRequest{
		Node:   o.node.ID,
		Action: accessibility.SetValue,
		Number: value,
	}) {
		return dbus.Errorf(dbus.NotSupported, "there is nothing to carry the change out")
	}
	return nil
}
