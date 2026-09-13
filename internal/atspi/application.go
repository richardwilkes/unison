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
	"os"
	"strings"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/dbus"
)

// rootObject is the object that represents the application as a whole. Its children are the windows, and the registry
// makes it a child of the desktop when the application joins the accessibility tree.
type rootObject struct {
	a *Adapter
}

// Interfaces implements [dbus.Object].
func (o *rootObject) Interfaces() []*dbus.Interface {
	return []*dbus.Interface{o.accessibleInterface(), o.applicationInterface()}
}

// accessibleInterface returns the org.a11y.atspi.Accessible interface of the application root.
func (o *rootObject) accessibleInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceAccessible,
		Methods: []*dbus.Method{
			{Name: "GetChildAtIndex", In: "i", Out: objectRefSignature, Handle: o.getChildAtIndex},
			{Name: "GetChildren", Out: objectRefArraySignature, Handle: o.getChildren},
			{Name: "GetIndexInParent", Out: "i", Handle: o.getIndexInParent},
			{Name: "GetRelationSet", Out: relationSetSignature, Handle: o.getRelationSet},
			{Name: "GetRole", Out: "u", Handle: o.getRole},
			{Name: "GetRoleName", Out: "s", Handle: o.getRoleName},
			{Name: "GetLocalizedRoleName", Out: "s", Handle: o.getRoleName},
			{Name: "GetState", Out: stateSignature, Handle: o.getState},
			{Name: "GetAttributes", Out: stringDictSignature, Handle: o.getAttributes},
			{Name: "GetApplication", Out: objectRefSignature, Handle: o.getApplication},
			{Name: "GetInterfaces", Out: "as", Handle: o.getInterfaces},
		},
		Properties: []*dbus.Property{
			{Name: "Name", Sig: "s", Get: func() (any, error) { return applicationName(), nil }},
			{Name: "Description", Sig: "s", Get: func() (any, error) { return "", nil }},
			{
				Name: "Parent",
				Sig:  objectRefSignature,
				Get:  func() (any, error) { return o.a.desktopReference(), nil },
			},
			{Name: "ChildCount", Sig: "i", Get: func() (any, error) { return int32(len(o.windows())), nil }},
			{Name: "Locale", Sig: "s", Get: func() (any, error) { return currentLocale(), nil }},
			{Name: "AccessibleId", Sig: "s", Get: func() (any, error) { return "", nil }},
			{Name: "HelpText", Sig: "s", Get: func() (any, error) { return "", nil }},
		},
	}
}

// applicationInterface returns the org.a11y.atspi.Application interface, which says what toolkit the application is
// built with and holds the identifier the registry assigns to it.
func (o *rootObject) applicationInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceApplication,
		Methods: []*dbus.Method{
			{Name: "GetLocale", In: "u", Out: "s", Handle: o.getLocale},
			{Name: "GetApplicationBusAddress", Out: "s", Handle: o.getApplicationBusAddress},
		},
		Properties: []*dbus.Property{
			{Name: "ToolkitName", Sig: "s", Get: func() (any, error) { return toolkitName, nil }},
			{Name: "Version", Sig: "s", Get: func() (any, error) { return o.a.cfg.ToolkitVersion, nil }},
			{Name: "ToolkitVersion", Sig: "s", Get: func() (any, error) { return o.a.cfg.ToolkitVersion, nil }},
			{Name: "AtspiVersion", Sig: "s", Get: func() (any, error) { return atspiVersion, nil }},
			{
				Name: "Id",
				Sig:  "i",
				Get:  func() (any, error) { return o.a.appID.Load(), nil },
				Set:  o.setID,
			},
		},
	}
}

// windows returns the windows the application has published, in the order they became children of the root.
func (o *rootObject) windows() []*windowState {
	return o.a.windowOrder()
}

// windowReferences returns the references to the root objects of the application's windows.
func (o *rootObject) windowReferences() []dbus.ObjectRef {
	windows := o.windows()
	refs := make([]dbus.ObjectRef, 0, len(windows))
	for _, ws := range windows {
		if data := ws.data.Load(); data != nil {
			refs = append(refs, o.a.reference(data.tree.Root))
		}
	}
	return refs
}

// getChildAtIndex implements org.a11y.atspi.Accessible.GetChildAtIndex for the application root, whose children are its
// windows.
func (o *rootObject) getChildAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	refs := o.windowReferences()
	index := int(int32Arg(args, 0))
	if index < 0 || index >= len(refs) {
		call.Reply(nullReference())
		return
	}
	call.Reply(refs[index])
}

// getChildren implements org.a11y.atspi.Accessible.GetChildren for the application root.
func (o *rootObject) getChildren(call *dbus.Call) {
	call.Reply(o.windowReferences())
}

// getIndexInParent implements org.a11y.atspi.Accessible.GetIndexInParent for the application root. Where the
// application sits among the desktop's children is the registry's business, not the application's, so the answer is
// "unknown".
func (o *rootObject) getIndexInParent(call *dbus.Call) {
	call.Reply(int32(-1))
}

// getRelationSet implements org.a11y.atspi.Accessible.GetRelationSet for the application root, which has no relations.
func (o *rootObject) getRelationSet(call *dbus.Call) {
	call.Reply([]any{})
}

// getRole implements org.a11y.atspi.Accessible.GetRole for the application root.
func (o *rootObject) getRole(call *dbus.Call) {
	call.Reply(uint32(RoleApplication))
}

// getRoleName implements org.a11y.atspi.Accessible.GetRoleName and GetLocalizedRoleName for the application root.
func (o *rootObject) getRoleName(call *dbus.Call) {
	call.Reply(RoleName(RoleApplication))
}

// getState implements org.a11y.atspi.Accessible.GetState for the application root. An application is always there and
// always usable; the states that vary belong to its windows.
func (o *rootObject) getState(call *dbus.Call) {
	call.Reply(rootStates().Words())
}

// rootStates returns the states of the application root.
func rootStates() StateSet {
	var set StateSet
	return set.With(StateEnabled, StateSensitive, StateShowing, StateVisible)
}

// getAttributes implements org.a11y.atspi.Accessible.GetAttributes for the application root.
func (o *rootObject) getAttributes(call *dbus.Call) {
	call.Reply(dbus.Dict{{Key: toolkitAttribute, Value: toolkitName}})
}

// getApplication implements org.a11y.atspi.Accessible.GetApplication for the application root, which is itself.
func (o *rootObject) getApplication(call *dbus.Call) {
	call.Reply(o.a.rootReference())
}

// getInterfaces implements org.a11y.atspi.Accessible.GetInterfaces for the application root.
func (o *rootObject) getInterfaces(call *dbus.Call) {
	call.Reply(rootInterfaces())
}

// rootInterfaces returns the names of the interfaces the application root implements.
func rootInterfaces() []string {
	return []string{InterfaceAccessible, InterfaceApplication}
}

// getLocale implements org.a11y.atspi.Application.GetLocale. The argument names the category being asked about; Unison
// has one locale for all of them.
func (o *rootObject) getLocale(call *dbus.Call) {
	if _, ok := callArgs(call); !ok {
		return
	}
	call.Reply(currentLocale())
}

// getApplicationBusAddress implements org.a11y.atspi.Application.GetApplicationBusAddress. It is how an application
// offers a private bus of its own for the assistive technology to use instead of the accessibility bus, which Unison
// does not do. libatspi asks every application, so the answer has to be a polite refusal rather than an unknown method.
func (o *rootObject) getApplicationBusAddress(call *dbus.Call) {
	call.Error(dbus.NotSupported, "unison does not offer a private accessibility bus")
}

// setID records the identifier the registry assigns to the application. The registry sets it as soon as the application
// is embedded, and expects to be able to read it back.
func (o *rootObject) setID(v any) error {
	id, ok := v.(int32)
	if !ok {
		return dbus.Errorf(dbus.InvalidArgs, "an int32 is required")
	}
	o.a.appID.Store(id)
	return nil
}

// applicationName returns the name to report for the application, which is the one the application itself set.
func applicationName() string {
	return xos.AppName
}

// localeEnvKeys are the environment variables that say what locale the process is running in, in the order POSIX gives
// them precedence.
var localeEnvKeys = []string{"LC_ALL", "LC_MESSAGES", "LANG"}

// currentLocale returns the POSIX locale of the process, or "C" when the environment says nothing.
func currentLocale() string {
	for _, key := range localeEnvKeys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return "C"
}
