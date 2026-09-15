// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

// This file holds the UI Automation identifiers and enumerations the provider needs, plus the VARIANT type tags used
// to answer property requests. The names match those in the Windows SDK's uiautomationcoreapi.h, uiautomationcore.idl
// and wtypes.h so that each value can be checked against the SDK or Microsoft Learn without translation.
//
// Nothing here touches the OS, so the file carries no build constraint: the mapping logic in uia_map.go is portable
// and its tests run on every platform.

// PropertyID identifies one UI Automation property. A provider answers a request for one through
// IRawElementProviderSimple::GetPropertyValue and reports a change to one with UiaRaiseAutomationPropertyChangedEvent.
type PropertyID int32

// Element property identifiers. These apply to every element, regardless of which control patterns it supports.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-automation-element-propids
const (
	UIA_RuntimeIdPropertyId            PropertyID = 30000
	UIA_BoundingRectanglePropertyId    PropertyID = 30001
	UIA_ProcessIdPropertyId            PropertyID = 30002
	UIA_ControlTypePropertyId          PropertyID = 30003
	UIA_LocalizedControlTypePropertyId PropertyID = 30004
	UIA_NamePropertyId                 PropertyID = 30005
	UIA_AcceleratorKeyPropertyId       PropertyID = 30006
	UIA_AccessKeyPropertyId            PropertyID = 30007
	UIA_HasKeyboardFocusPropertyId     PropertyID = 30008
	UIA_IsKeyboardFocusablePropertyId  PropertyID = 30009
	UIA_IsEnabledPropertyId            PropertyID = 30010
	UIA_AutomationIdPropertyId         PropertyID = 30011
	UIA_ClassNamePropertyId            PropertyID = 30012
	UIA_HelpTextPropertyId             PropertyID = 30013
	UIA_IsControlElementPropertyId     PropertyID = 30016
	UIA_IsContentElementPropertyId     PropertyID = 30017
	UIA_LabeledByPropertyId            PropertyID = 30018
	UIA_IsPasswordPropertyId           PropertyID = 30019
	UIA_NativeWindowHandlePropertyId   PropertyID = 30020
	UIA_IsOffscreenPropertyId          PropertyID = 30022
	UIA_OrientationPropertyId          PropertyID = 30023
	UIA_FrameworkIdPropertyId          PropertyID = 30024
	UIA_ItemStatusPropertyId           PropertyID = 30026
	UIA_IsDataValidForFormPropertyId   PropertyID = 30103
	UIA_ControllerForPropertyId        PropertyID = 30104
	UIA_DescribedByPropertyId          PropertyID = 30105
	UIA_ProviderDescriptionPropertyId  PropertyID = 30107
	UIA_LiveSettingPropertyId          PropertyID = 30135
	UIA_PositionInSetPropertyId        PropertyID = 30152
	UIA_SizeOfSetPropertyId            PropertyID = 30153
	UIA_LevelPropertyId                PropertyID = 30154
	UIA_FullDescriptionPropertyId      PropertyID = 30159
	UIA_HeadingLevelPropertyId         PropertyID = 30173
	UIA_IsDialogPropertyId             PropertyID = 30174
)

// Control pattern property identifiers. Each belongs to one control pattern and is meaningful only on an element that
// supports that pattern, so a property-changed event for one must never be raised on an element that does not.
//
// That rule leaves no way to say a pattern has gone away, since the element it went away from is exactly the element
// the pattern's own properties may not be raised on. The pattern availability properties below are what say it, and
// UIADecideRaises raises one of those instead: an element that has lost a pattern reports the loss through the
// availability property and reports nothing at all through the pattern's own. See uiaDecider.patternAvailability.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-control-pattern-propids
const (
	UIA_ValueValuePropertyId                        PropertyID = 30045
	UIA_ValueIsReadOnlyPropertyId                   PropertyID = 30046
	UIA_RangeValueValuePropertyId                   PropertyID = 30047
	UIA_RangeValueIsReadOnlyPropertyId              PropertyID = 30048
	UIA_RangeValueMinimumPropertyId                 PropertyID = 30049
	UIA_RangeValueMaximumPropertyId                 PropertyID = 30050
	UIA_RangeValueLargeChangePropertyId             PropertyID = 30051
	UIA_RangeValueSmallChangePropertyId             PropertyID = 30052
	UIA_SelectionSelectionPropertyId                PropertyID = 30059
	UIA_SelectionCanSelectMultiplePropertyId        PropertyID = 30060
	UIA_SelectionIsSelectionRequiredPropertyId      PropertyID = 30061
	UIA_GridRowCountPropertyId                      PropertyID = 30062
	UIA_GridColumnCountPropertyId                   PropertyID = 30063
	UIA_GridItemRowPropertyId                       PropertyID = 30064
	UIA_GridItemColumnPropertyId                    PropertyID = 30065
	UIA_GridItemRowSpanPropertyId                   PropertyID = 30066
	UIA_GridItemColumnSpanPropertyId                PropertyID = 30067
	UIA_GridItemContainingGridPropertyId            PropertyID = 30068
	UIA_ExpandCollapseExpandCollapseStatePropertyId PropertyID = 30070
	UIA_WindowCanMaximizePropertyId                 PropertyID = 30073
	UIA_WindowCanMinimizePropertyId                 PropertyID = 30074
	UIA_WindowWindowVisualStatePropertyId           PropertyID = 30075
	UIA_WindowWindowInteractionStatePropertyId      PropertyID = 30076
	UIA_WindowIsModalPropertyId                     PropertyID = 30077
	UIA_WindowIsTopmostPropertyId                   PropertyID = 30078
	UIA_SelectionItemIsSelectedPropertyId           PropertyID = 30079
	UIA_SelectionItemSelectionContainerPropertyId   PropertyID = 30080
	UIA_TableRowHeadersPropertyId                   PropertyID = 30081
	UIA_TableColumnHeadersPropertyId                PropertyID = 30082
	UIA_TableRowOrColumnMajorPropertyId             PropertyID = 30083
	UIA_TableItemRowHeaderItemsPropertyId           PropertyID = 30084
	UIA_TableItemColumnHeaderItemsPropertyId        PropertyID = 30085
	UIA_ToggleToggleStatePropertyId                 PropertyID = 30086
)

// Control pattern availability property identifiers. Each reports whether one control pattern is available on an
// element, and unlike the pattern properties above every element answers one: false is precisely the answer for an
// element that does not support the pattern, which is what makes these the only properties that may be raised when a
// pattern appears or vanishes.
//
// Nothing answers one through GetPropertyValue. UI Automation works a client's own request for one out by asking the
// provider for the pattern itself, so a provider that answered here as well would only be repeating what
// GetPatternProvider already says; what a provider must do is raise the change, since a client caches pattern
// availability and would otherwise go on asking an element for a pattern it no longer hands out.
//
// Only the patterns this package implements are listed, for the reason PatternSet gives: a name here for a pattern the
// provider never hands out would be a claim about behavior that does not exist.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-automation-element-propids
const (
	UIA_IsExpandCollapsePatternAvailablePropertyId PropertyID = 30028
	UIA_IsGridItemPatternAvailablePropertyId       PropertyID = 30029
	UIA_IsGridPatternAvailablePropertyId           PropertyID = 30030
	UIA_IsInvokePatternAvailablePropertyId         PropertyID = 30031
	UIA_IsRangeValuePatternAvailablePropertyId     PropertyID = 30033
	UIA_IsScrollItemPatternAvailablePropertyId     PropertyID = 30035
	UIA_IsSelectionItemPatternAvailablePropertyId  PropertyID = 30036
	UIA_IsSelectionPatternAvailablePropertyId      PropertyID = 30037
	UIA_IsTablePatternAvailablePropertyId          PropertyID = 30038
	UIA_IsTableItemPatternAvailablePropertyId      PropertyID = 30039
	UIA_IsTogglePatternAvailablePropertyId         PropertyID = 30041
	UIA_IsValuePatternAvailablePropertyId          PropertyID = 30043
	UIA_IsWindowPatternAvailablePropertyId         PropertyID = 30044
)

// ControlTypeID identifies what kind of control an element is. It is the value of UIA_ControlTypePropertyId and is the
// single largest influence on how a screen reader describes an element, since the client derives the spoken control
// type, the default patterns it looks for, and its treatment in the control and content views from it.
type ControlTypeID int32

// Control type identifiers.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids
const (
	UIA_ButtonControlTypeId      ControlTypeID = 50000
	UIA_CheckBoxControlTypeId    ControlTypeID = 50002
	UIA_ComboBoxControlTypeId    ControlTypeID = 50003
	UIA_EditControlTypeId        ControlTypeID = 50004
	UIA_HyperlinkControlTypeId   ControlTypeID = 50005
	UIA_ImageControlTypeId       ControlTypeID = 50006
	UIA_ListItemControlTypeId    ControlTypeID = 50007
	UIA_ListControlTypeId        ControlTypeID = 50008
	UIA_MenuControlTypeId        ControlTypeID = 50009
	UIA_MenuBarControlTypeId     ControlTypeID = 50010
	UIA_MenuItemControlTypeId    ControlTypeID = 50011
	UIA_ProgressBarControlTypeId ControlTypeID = 50012
	UIA_RadioButtonControlTypeId ControlTypeID = 50013
	UIA_ScrollBarControlTypeId   ControlTypeID = 50014
	UIA_SliderControlTypeId      ControlTypeID = 50015
	UIA_SpinnerControlTypeId     ControlTypeID = 50016
	UIA_TabControlTypeId         ControlTypeID = 50018
	UIA_TabItemControlTypeId     ControlTypeID = 50019
	UIA_TextControlTypeId        ControlTypeID = 50020
	UIA_ToolBarControlTypeId     ControlTypeID = 50021
	UIA_ToolTipControlTypeId     ControlTypeID = 50022
	UIA_TreeControlTypeId        ControlTypeID = 50023
	UIA_TreeItemControlTypeId    ControlTypeID = 50024
	UIA_CustomControlTypeId      ControlTypeID = 50025
	UIA_GroupControlTypeId       ControlTypeID = 50026
	UIA_DataGridControlTypeId    ControlTypeID = 50028
	UIA_DataItemControlTypeId    ControlTypeID = 50029
	UIA_DocumentControlTypeId    ControlTypeID = 50030
	UIA_WindowControlTypeId      ControlTypeID = 50032
	UIA_PaneControlTypeId        ControlTypeID = 50033
	UIA_HeaderControlTypeId      ControlTypeID = 50034
	UIA_HeaderItemControlTypeId  ControlTypeID = 50035
	UIA_TableControlTypeId       ControlTypeID = 50036
	UIA_SeparatorControlTypeId   ControlTypeID = 50038
)

// PatternID identifies one UI Automation control pattern. A client asks for a pattern through
// IRawElementProviderSimple::GetPatternProvider, which must return the interface for the pattern or nil.
type PatternID int32

// Control pattern identifiers. Only the patterns this package implements, or expects to implement, are listed.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controlpattern-ids
const (
	UIA_InvokePatternId         PatternID = 10000
	UIA_SelectionPatternId      PatternID = 10001
	UIA_ValuePatternId          PatternID = 10002
	UIA_RangeValuePatternId     PatternID = 10003
	UIA_ScrollPatternId         PatternID = 10004
	UIA_ExpandCollapsePatternId PatternID = 10005
	UIA_GridPatternId           PatternID = 10006
	UIA_GridItemPatternId       PatternID = 10007
	UIA_WindowPatternId         PatternID = 10009
	UIA_SelectionItemPatternId  PatternID = 10010
	UIA_TablePatternId          PatternID = 10012
	UIA_TableItemPatternId      PatternID = 10013
	UIA_TextPatternId           PatternID = 10014
	UIA_TogglePatternId         PatternID = 10015
	UIA_ScrollItemPatternId     PatternID = 10017
	UIA_TextPattern2Id          PatternID = 10024
)

// EventID identifies one UI Automation event. A provider announces one with UiaRaiseAutomationEvent, except for the
// property-changed, structure-changed and notification events, which have dedicated functions.
type EventID int32

// Event identifiers.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-event-ids
const (
	UIA_ToolTipOpenedEventId                             EventID = 20000
	UIA_ToolTipClosedEventId                             EventID = 20001
	UIA_StructureChangedEventId                          EventID = 20002
	UIA_MenuOpenedEventId                                EventID = 20003
	UIA_AutomationPropertyChangedEventId                 EventID = 20004
	UIA_AutomationFocusChangedEventId                    EventID = 20005
	UIA_MenuClosedEventId                                EventID = 20007
	UIA_Invoke_InvokedEventId                            EventID = 20009
	UIA_SelectionItem_ElementAddedToSelectionEventId     EventID = 20010
	UIA_SelectionItem_ElementRemovedFromSelectionEventId EventID = 20011
	UIA_SelectionItem_ElementSelectedEventId             EventID = 20012
	UIA_Text_TextSelectionChangedEventId                 EventID = 20014
	UIA_Text_TextChangedEventId                          EventID = 20015
	UIA_Window_WindowOpenedEventId                       EventID = 20016
	UIA_Window_WindowClosedEventId                       EventID = 20017
	UIA_LiveRegionChangedEventId                         EventID = 20024
	UIA_NotificationEventId                              EventID = 20035
)

// ProviderOptions describes how a provider expects to be called. It is the value returned by
// IRawElementProviderSimple::get_ProviderOptions.
type ProviderOptions int32

// Possible ProviderOptions values. The providers in this package report ProviderOptions_ServerSideProvider alone:
// deliberately without ProviderOptions_UseComThreading, since every provider method answers from an immutable snapshot
// and may therefore run on whichever thread UI Automation calls in on. Asking for COM threading would require the UI
// thread to pump COM messages, which it cannot do while it sits in a modal loop or inside DoDragDrop.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-provideroptions
const (
	ProviderOptions_ClientSideProvider    ProviderOptions = 0x1
	ProviderOptions_ServerSideProvider    ProviderOptions = 0x2
	ProviderOptions_NonClientAreaProvider ProviderOptions = 0x4
	ProviderOptions_OverrideProvider      ProviderOptions = 0x8
	ProviderOptions_ProviderOwnsSetFocus  ProviderOptions = 0x10
	ProviderOptions_UseComThreading       ProviderOptions = 0x20
)

// NavigateDirection is the direction IRawElementProviderFragment::Navigate is asked to move in.
type NavigateDirection int32

// Possible NavigateDirection values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-navigatedirection
const (
	NavigateDirection_Parent NavigateDirection = iota
	NavigateDirection_NextSibling
	NavigateDirection_PreviousSibling
	NavigateDirection_FirstChild
	NavigateDirection_LastChild
)

// StructureChangeType describes how an element's place in the tree changed, for
// UiaRaiseStructureChangedEvent.
type StructureChangeType int32

// Possible StructureChangeType values. Only the three this package raises are defined.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-structurechangetype
const (
	StructureChangeType_ChildAdded StructureChangeType = iota
	StructureChangeType_ChildRemoved
	StructureChangeType_ChildrenInvalidated
)

// ToggleState is the state of an element that supports the Toggle pattern.
type ToggleState int32

// Possible ToggleState values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-togglestate
const (
	ToggleState_Off ToggleState = iota
	ToggleState_On
	ToggleState_Indeterminate
)

// ExpandCollapseState is the state of an element that supports the ExpandCollapse pattern.
type ExpandCollapseState int32

// Possible ExpandCollapseState values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-expandcollapsestate
const (
	ExpandCollapseState_Collapsed ExpandCollapseState = iota
	ExpandCollapseState_Expanded
	ExpandCollapseState_PartiallyExpanded
	ExpandCollapseState_LeafNode
)

// WindowVisualState is how a window that supports the Window pattern is currently displayed.
type WindowVisualState int32

// Possible WindowVisualState values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-windowvisualstate
const (
	WindowVisualState_Normal WindowVisualState = iota
	WindowVisualState_Maximized
	WindowVisualState_Minimized
)

// WindowInteractionState is how ready a window that supports the Window pattern is to accept input.
type WindowInteractionState int32

// Possible WindowInteractionState values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-windowinteractionstate
const (
	WindowInteractionState_Running WindowInteractionState = iota
	WindowInteractionState_Closing
	WindowInteractionState_ReadyForUserInteraction
	WindowInteractionState_BlockedByModalWindow
	WindowInteractionState_NotResponding
)

// OrientationType is the axis an element's value varies along. It is the value of UIA_OrientationPropertyId.
type OrientationType int32

// Possible OrientationType values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-orientationtype
const (
	OrientationType_None OrientationType = iota
	OrientationType_Horizontal
	OrientationType_Vertical
)

// NotificationKind says what sort of change a notification event describes.
type NotificationKind int32

// Possible NotificationKind values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-notificationkind
const (
	NotificationKind_ItemAdded NotificationKind = iota
	NotificationKind_ItemRemoved
	NotificationKind_ActionCompleted
	NotificationKind_ActionAborted
	NotificationKind_Other
)

// NotificationProcessing tells the client how to order a notification against the ones already queued.
type NotificationProcessing int32

// Possible NotificationProcessing values. NotificationProcessing_All is the one this package uses: every announcement
// the application makes is spoken, in order, rather than superseding an earlier one.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-notificationprocessing
const (
	NotificationProcessing_ImportantAll NotificationProcessing = iota
	NotificationProcessing_ImportantMostRecent
	NotificationProcessing_All
	NotificationProcessing_MostRecent
	NotificationProcessing_CurrentThenMostRecent
)

// LiveSetting is how aggressively a client should announce changes within an element. It is the value of
// UIA_LiveSettingPropertyId.
//
// No element here answers that property. A client consults it only for an element it has been given a
// UIA_LiveRegionChangedEventId for, and this package raises that for nothing: an announcement goes out as a
// notification event, which carries its own ordering instead.
type LiveSetting int32

// Possible LiveSetting values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-livesetting
const (
	LiveSetting_Off LiveSetting = iota
	LiveSetting_Polite
	LiveSetting_Assertive
)

// RowOrColumnMajor is the direction items in a table are traversed in. It is the value of
// UIA_TableRowOrColumnMajorPropertyId.
type RowOrColumnMajor int32

// Possible RowOrColumnMajor values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-roworcolumnmajor
const (
	RowOrColumnMajor_RowMajor RowOrColumnMajor = iota
	RowOrColumnMajor_ColumnMajor
	RowOrColumnMajor_Indeterminate
)

// HeadingLevelID is the value of UIA_HeadingLevelPropertyId. Unlike every other level in UI Automation it is not a
// plain integer: the nine levels have their own identifiers, and an element that is not a heading reports
// HeadingLevel_None rather than zero.
type HeadingLevelID int32

// Possible HeadingLevelID values.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-automation-element-propids
const (
	HeadingLevel_None HeadingLevelID = 80050
	HeadingLevel1     HeadingLevelID = 80051
	HeadingLevel2     HeadingLevelID = 80052
	HeadingLevel3     HeadingLevelID = 80053
	HeadingLevel4     HeadingLevelID = 80054
	HeadingLevel5     HeadingLevelID = 80055
	HeadingLevel6     HeadingLevelID = 80056
	HeadingLevel7     HeadingLevelID = 80057
	HeadingLevel8     HeadingLevelID = 80058
	HeadingLevel9     HeadingLevelID = 80059
)

const (
	// UiaRootObjectId is the lParam value a WM_GETOBJECT message carries when the caller wants the window's UI
	// Automation fragment root. Any other value is a request for a different accessibility interface (Microsoft Active
	// Accessibility, for instance) and must be refused so that the default window procedure can answer it.
	UiaRootObjectId int32 = -25
	// UiaAppendRuntimeId is the first element of a runtime identifier that is relative to the fragment root's own
	// runtime identifier, which UI Automation prepends. Every non-root element in a fragment uses it, so that runtime
	// identifiers stay unique across the windows of a process without the provider having to know the window's own
	// identifier.
	UiaAppendRuntimeId int32 = 3
	// UiaFrameworkID is the value this provider reports for UIA_FrameworkIdPropertyId, naming the toolkit that drew
	// the element. Clients use it only for diagnostics and for framework-specific workarounds.
	UiaFrameworkID = "Unison"
)

// UI Automation HRESULT values. A provider returns one of these from a method it cannot satisfy; the client turns it
// back into the corresponding managed or scripting error.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-error-codes
const (
	UIA_E_ELEMENTNOTENABLED   uint64 = 0x80040200
	UIA_E_ELEMENTNOTAVAILABLE uint64 = 0x80040201
	UIA_E_NOTSUPPORTED        uint64 = 0x80040204
	UIA_E_INVALIDOPERATION    uint64 = 0x80131509
)

// VARTYPE is the type tag of a VARIANT, held in its VT field.
type VARTYPE uint16

// The VARIANT type tags this package uses. VT_ARRAY is a flag combined with the element type, so a SAFEARRAY of
// IUnknown pointers is VT_ARRAY|VT_UNKNOWN.
//
// https://learn.microsoft.com/en-us/windows/win32/api/wtypes/ne-wtypes-varenum
const (
	VT_EMPTY   VARTYPE = 0
	VT_I4      VARTYPE = 3
	VT_R8      VARTYPE = 5
	VT_BSTR    VARTYPE = 8
	VT_BOOL    VARTYPE = 11
	VT_UNKNOWN VARTYPE = 13
	VT_ARRAY   VARTYPE = 0x2000
)

// VARIANT_BOOL values. A VARIANT_BOOL is a 16-bit signed integer in which true is every bit set rather than one, so a
// plain 1 is neither true nor false to most COM clients.
const (
	VARIANT_TRUE  int16 = -1
	VARIANT_FALSE int16 = 0
)
