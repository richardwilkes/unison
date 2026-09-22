// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

// This file holds the UI Automation identifiers and enumerations the provider needs, plus the VARIANT type tags used
// to answer property requests. The names match those in the Windows SDK's uiautomationcoreapi.h, uiautomationcore.idl
// and wtypes.h so that each value can be checked against the SDK or Microsoft Learn without translation.
//
// Nothing here touches the OS, so the file carries no build constraint: the mapping logic in map.go is portable
// and its tests run on every platform.

// PropertyID identifies one UI Automation property. A provider answers a request for one through
// IRawElementProviderSimple::GetPropertyValue and reports a change to one with RaiseAutomationPropertyChangedEvent.
type PropertyID int32

// Element property identifiers. These apply to every element, regardless of which control patterns it supports.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-automation-element-propids
const (
	RuntimeIdPropertyId            PropertyID = 30000
	BoundingRectanglePropertyId    PropertyID = 30001
	ProcessIdPropertyId            PropertyID = 30002
	ControlTypePropertyId          PropertyID = 30003
	LocalizedControlTypePropertyId PropertyID = 30004
	NamePropertyId                 PropertyID = 30005
	AcceleratorKeyPropertyId       PropertyID = 30006
	AccessKeyPropertyId            PropertyID = 30007
	HasKeyboardFocusPropertyId     PropertyID = 30008
	IsKeyboardFocusablePropertyId  PropertyID = 30009
	IsEnabledPropertyId            PropertyID = 30010
	AutomationIdPropertyId         PropertyID = 30011
	ClassNamePropertyId            PropertyID = 30012
	HelpTextPropertyId             PropertyID = 30013
	IsControlElementPropertyId     PropertyID = 30016
	IsContentElementPropertyId     PropertyID = 30017
	LabeledByPropertyId            PropertyID = 30018
	IsPasswordPropertyId           PropertyID = 30019
	NativeWindowHandlePropertyId   PropertyID = 30020
	IsOffscreenPropertyId          PropertyID = 30022
	OrientationPropertyId          PropertyID = 30023
	FrameworkIdPropertyId          PropertyID = 30024
	ItemStatusPropertyId           PropertyID = 30026
	IsDataValidForFormPropertyId   PropertyID = 30103
	ControllerForPropertyId        PropertyID = 30104
	DescribedByPropertyId          PropertyID = 30105
	ProviderDescriptionPropertyId  PropertyID = 30107
	LiveSettingPropertyId          PropertyID = 30135
	PositionInSetPropertyId        PropertyID = 30152
	SizeOfSetPropertyId            PropertyID = 30153
	LevelPropertyId                PropertyID = 30154
	FullDescriptionPropertyId      PropertyID = 30159
	HeadingLevelPropertyId         PropertyID = 30173
	IsDialogPropertyId             PropertyID = 30174
)

// Control pattern property identifiers. Each belongs to one control pattern and is meaningful only on an element that
// supports that pattern, so a property-changed event for one must never be raised on an element that does not.
//
// That rule leaves no way to say a pattern has gone away, since the element it went away from is exactly the element
// the pattern's own properties may not be raised on. The pattern availability properties below are what say it, and
// DecideRaises raises one of those instead: an element that has lost a pattern reports the loss through the
// availability property and reports nothing at all through the pattern's own. See decider.patternAvailability.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-control-pattern-propids
const (
	ValueValuePropertyId                        PropertyID = 30045
	ValueIsReadOnlyPropertyId                   PropertyID = 30046
	RangeValueValuePropertyId                   PropertyID = 30047
	RangeValueIsReadOnlyPropertyId              PropertyID = 30048
	RangeValueMinimumPropertyId                 PropertyID = 30049
	RangeValueMaximumPropertyId                 PropertyID = 30050
	RangeValueLargeChangePropertyId             PropertyID = 30051
	RangeValueSmallChangePropertyId             PropertyID = 30052
	SelectionSelectionPropertyId                PropertyID = 30059
	SelectionCanSelectMultiplePropertyId        PropertyID = 30060
	SelectionIsSelectionRequiredPropertyId      PropertyID = 30061
	GridRowCountPropertyId                      PropertyID = 30062
	GridColumnCountPropertyId                   PropertyID = 30063
	GridItemRowPropertyId                       PropertyID = 30064
	GridItemColumnPropertyId                    PropertyID = 30065
	GridItemRowSpanPropertyId                   PropertyID = 30066
	GridItemColumnSpanPropertyId                PropertyID = 30067
	GridItemContainingGridPropertyId            PropertyID = 30068
	ExpandCollapseExpandCollapseStatePropertyId PropertyID = 30070
	WindowCanMaximizePropertyId                 PropertyID = 30073
	WindowCanMinimizePropertyId                 PropertyID = 30074
	WindowWindowVisualStatePropertyId           PropertyID = 30075
	WindowWindowInteractionStatePropertyId      PropertyID = 30076
	WindowIsModalPropertyId                     PropertyID = 30077
	WindowIsTopmostPropertyId                   PropertyID = 30078
	SelectionItemIsSelectedPropertyId           PropertyID = 30079
	SelectionItemSelectionContainerPropertyId   PropertyID = 30080
	TableRowHeadersPropertyId                   PropertyID = 30081
	TableColumnHeadersPropertyId                PropertyID = 30082
	TableRowOrColumnMajorPropertyId             PropertyID = 30083
	TableItemRowHeaderItemsPropertyId           PropertyID = 30084
	TableItemColumnHeaderItemsPropertyId        PropertyID = 30085
	ToggleToggleStatePropertyId                 PropertyID = 30086
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
	IsExpandCollapsePatternAvailablePropertyId PropertyID = 30028
	IsGridItemPatternAvailablePropertyId       PropertyID = 30029
	IsGridPatternAvailablePropertyId           PropertyID = 30030
	IsInvokePatternAvailablePropertyId         PropertyID = 30031
	IsRangeValuePatternAvailablePropertyId     PropertyID = 30033
	IsScrollItemPatternAvailablePropertyId     PropertyID = 30035
	IsSelectionItemPatternAvailablePropertyId  PropertyID = 30036
	IsSelectionPatternAvailablePropertyId      PropertyID = 30037
	IsTablePatternAvailablePropertyId          PropertyID = 30038
	IsTableItemPatternAvailablePropertyId      PropertyID = 30039
	IsTextPatternAvailablePropertyId           PropertyID = 30040
	IsTogglePatternAvailablePropertyId         PropertyID = 30041
	IsValuePatternAvailablePropertyId          PropertyID = 30043
	IsWindowPatternAvailablePropertyId         PropertyID = 30044
	IsTextPattern2AvailablePropertyId          PropertyID = 30119
	IsTextChildPatternAvailablePropertyId      PropertyID = 30136
)

// ControlTypeID identifies what kind of control an element is. It is the value of ControlTypePropertyId and is the
// single largest influence on how a screen reader describes an element, since the client derives the spoken control
// type, the default patterns it looks for, and its treatment in the control and content views from it.
type ControlTypeID int32

// Control type identifiers.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controltype-ids
const (
	ButtonControlTypeId      ControlTypeID = 50000
	CheckBoxControlTypeId    ControlTypeID = 50002
	ComboBoxControlTypeId    ControlTypeID = 50003
	EditControlTypeId        ControlTypeID = 50004
	HyperlinkControlTypeId   ControlTypeID = 50005
	ImageControlTypeId       ControlTypeID = 50006
	ListItemControlTypeId    ControlTypeID = 50007
	ListControlTypeId        ControlTypeID = 50008
	MenuControlTypeId        ControlTypeID = 50009
	MenuBarControlTypeId     ControlTypeID = 50010
	MenuItemControlTypeId    ControlTypeID = 50011
	ProgressBarControlTypeId ControlTypeID = 50012
	RadioButtonControlTypeId ControlTypeID = 50013
	ScrollBarControlTypeId   ControlTypeID = 50014
	SliderControlTypeId      ControlTypeID = 50015
	SpinnerControlTypeId     ControlTypeID = 50016
	TabControlTypeId         ControlTypeID = 50018
	TabItemControlTypeId     ControlTypeID = 50019
	TextControlTypeId        ControlTypeID = 50020
	ToolBarControlTypeId     ControlTypeID = 50021
	ToolTipControlTypeId     ControlTypeID = 50022
	TreeControlTypeId        ControlTypeID = 50023
	TreeItemControlTypeId    ControlTypeID = 50024
	CustomControlTypeId      ControlTypeID = 50025
	GroupControlTypeId       ControlTypeID = 50026
	DataGridControlTypeId    ControlTypeID = 50028
	DataItemControlTypeId    ControlTypeID = 50029
	DocumentControlTypeId    ControlTypeID = 50030
	WindowControlTypeId      ControlTypeID = 50032
	PaneControlTypeId        ControlTypeID = 50033
	HeaderControlTypeId      ControlTypeID = 50034
	HeaderItemControlTypeId  ControlTypeID = 50035
	TableControlTypeId       ControlTypeID = 50036
	SeparatorControlTypeId   ControlTypeID = 50038
)

// PatternID identifies one UI Automation control pattern. A client asks for a pattern through
// IRawElementProviderSimple::GetPatternProvider, which must return the interface for the pattern or nil.
type PatternID int32

// Control pattern identifiers. Only the patterns this package implements, or expects to implement, are listed.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-controlpattern-ids
const (
	InvokePatternId         PatternID = 10000
	SelectionPatternId      PatternID = 10001
	ValuePatternId          PatternID = 10002
	RangeValuePatternId     PatternID = 10003
	ScrollPatternId         PatternID = 10004
	ExpandCollapsePatternId PatternID = 10005
	GridPatternId           PatternID = 10006
	GridItemPatternId       PatternID = 10007
	WindowPatternId         PatternID = 10009
	SelectionItemPatternId  PatternID = 10010
	TablePatternId          PatternID = 10012
	TableItemPatternId      PatternID = 10013
	TextPatternId           PatternID = 10014
	TogglePatternId         PatternID = 10015
	ScrollItemPatternId     PatternID = 10017
	TextPattern2Id          PatternID = 10024
	TextChildPatternId      PatternID = 10029
)

// EventID identifies one UI Automation event. A provider announces one with RaiseAutomationEvent, except for the
// property-changed, structure-changed and notification events, which have dedicated functions.
type EventID int32

// Event identifiers.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-event-ids
const (
	ToolTipOpenedEventId                             EventID = 20000
	ToolTipClosedEventId                             EventID = 20001
	StructureChangedEventId                          EventID = 20002
	MenuOpenedEventId                                EventID = 20003
	AutomationPropertyChangedEventId                 EventID = 20004
	AutomationFocusChangedEventId                    EventID = 20005
	MenuClosedEventId                                EventID = 20007
	Invoke_InvokedEventId                            EventID = 20009
	SelectionItem_ElementAddedToSelectionEventId     EventID = 20010
	SelectionItem_ElementRemovedFromSelectionEventId EventID = 20011
	SelectionItem_ElementSelectedEventId             EventID = 20012
	Text_TextSelectionChangedEventId                 EventID = 20014
	Text_TextChangedEventId                          EventID = 20015
	Window_WindowOpenedEventId                       EventID = 20016
	Window_WindowClosedEventId                       EventID = 20017
	LiveRegionChangedEventId                         EventID = 20024
	NotificationEventId                              EventID = 20035
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
// RaiseStructureChangedEvent.
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

// OrientationType is the axis an element's value varies along. It is the value of OrientationPropertyId.
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
// LiveSettingPropertyId.
//
// No element here answers that property. A client consults it only for an element it has been given a
// LiveRegionChangedEventId for, and this package raises that for nothing: an announcement goes out as a
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
// TableRowOrColumnMajorPropertyId.
type RowOrColumnMajor int32

// Possible RowOrColumnMajor values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-roworcolumnmajor
const (
	RowOrColumnMajor_RowMajor RowOrColumnMajor = iota
	RowOrColumnMajor_ColumnMajor
	RowOrColumnMajor_Indeterminate
)

// HeadingLevelID is the value of HeadingLevelPropertyId. Unlike every other level in UI Automation it is not a
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

// TextUnit is the granularity a Text pattern range is expanded or moved by. It is what a screen reader reading a
// document by character, word or line asks for, so the boundaries each one names are the whole of how a document reads;
// see textDocument in text.go, which defines them.
type TextUnit int32

// Possible TextUnit values. Page is not a unit any document here divides into — nothing paginates — so it answers as
// Document does, which is the documented fallback for a provider that does not support a unit: report the next larger
// one it does.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-textunit
const (
	TextUnit_Character TextUnit = iota
	TextUnit_Format
	TextUnit_Word
	TextUnit_Line
	TextUnit_Paragraph
	TextUnit_Page
	TextUnit_Document
)

// TextPatternRangeEndpoint names one end of a text range, for the methods that move or compare a single endpoint.
type TextPatternRangeEndpoint int32

// Possible TextPatternRangeEndpoint values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-textpatternrangeendpoint
const (
	TextPatternRangeEndpoint_Start TextPatternRangeEndpoint = iota
	TextPatternRangeEndpoint_End
)

// SupportedTextSelection says what kind of selection a text container allows, and is the value of
// ITextProvider::get_SupportedTextSelection.
type SupportedTextSelection int32

// Possible SupportedTextSelection values. An element here reports Single while it accepts a selection at all and None
// otherwise: a label never accepts one, so a client must not be told it can place one.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-supportedtextselection
const (
	SupportedTextSelection_None SupportedTextSelection = iota
	SupportedTextSelection_Single
	SupportedTextSelection_Multiple
)

// TextAttributeID identifies one attribute of a run of text, which a client reads with
// ITextRangeProvider::GetAttributeValue and searches by with ITextRangeProvider::FindAttribute.
type TextAttributeID int32

// The text attributes this package answers, plus the two it recognizes and deliberately refuses.
//
// Culture and AnnotationTypes are listed because a screen reader asks for them of every run and the answer has to be
// the reserved not-supported value rather than a made-up one: a snapshot records neither the language a run is written
// in nor any annotation over it, and reporting a culture of zero would have a client read the text in the wrong voice.
//
// StyleName is answered only for a code block, whose style has no identifier of its own — StyleId_Custom says exactly
// that, and the name is what tells a client which custom style it is.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-textattribute-ids
const (
	CultureAttributeId            TextAttributeID = 40004
	FontNameAttributeId           TextAttributeID = 40005
	FontSizeAttributeId           TextAttributeID = 40006
	FontWeightAttributeId         TextAttributeID = 40007
	IsHiddenAttributeId           TextAttributeID = 40013
	IsItalicAttributeId           TextAttributeID = 40014
	IsReadOnlyAttributeId         TextAttributeID = 40015
	StrikethroughStyleAttributeId TextAttributeID = 40026
	UnderlineStyleAttributeId     TextAttributeID = 40030
	AnnotationTypesAttributeId    TextAttributeID = 40031
	StyleNameAttributeId          TextAttributeID = 40033
	StyleIdAttributeId            TextAttributeID = 40034
	LinkAttributeId               TextAttributeID = 40035
	IsActiveAttributeId           TextAttributeID = 40036
)

// TextDecorationLineStyle is the value of the UnderlineStyle and StrikethroughStyle attributes. Only the two states a
// snapshot records are defined: a run is underlined or struck through, or it is not, and UI Automation's other dozen
// styles say how the line is drawn, which nothing here knows.
type TextDecorationLineStyle int32

// Possible TextDecorationLineStyle values.
//
// https://learn.microsoft.com/en-us/windows/win32/api/uiautomationcore/ne-uiautomationcore-textdecorationlinestyle
const (
	TextDecorationLineStyle_None   TextDecorationLineStyle = 0
	TextDecorationLineStyle_Single TextDecorationLineStyle = 1
)

// StyleID is the value of the StyleId text attribute: which of the styles a word processor names a passage is drawn in.
// It is how a client walking a document by heading finds one — a heading's runs report Heading1 through Heading9 — and
// how a quotation, a list item and a code block are told apart from body text.
type StyleID int32

// The StyleId values UI Automation defines. The whole enumeration is listed rather than only the styles a document here
// answers with — styleAt produces Custom, Heading1 through Heading9, Quote, BulletedList and Normal — because these are
// a client's vocabulary as much as this provider's: FindAttribute takes a StyleId to search for, and a client is
// entitled to ask for Title or NumberedList and be told that no stretch of the document has it. A name for a value a
// client may pass in is not the claim about behavior that an unimplemented pattern name would be.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-style-ids
const (
	StyleId_Custom       StyleID = 70000
	StyleId_Heading1     StyleID = 70001
	StyleId_Heading2     StyleID = 70002
	StyleId_Heading3     StyleID = 70003
	StyleId_Heading4     StyleID = 70004
	StyleId_Heading5     StyleID = 70005
	StyleId_Heading6     StyleID = 70006
	StyleId_Heading7     StyleID = 70007
	StyleId_Heading8     StyleID = 70008
	StyleId_Heading9     StyleID = 70009
	StyleId_Title        StyleID = 70010
	StyleId_Subtitle     StyleID = 70011
	StyleId_Normal       StyleID = 70012
	StyleId_Emphasis     StyleID = 70013
	StyleId_Quote        StyleID = 70014
	StyleId_BulletedList StyleID = 70015
	StyleId_NumberedList StyleID = 70016
)

const (
	// RootObjectId is the lParam value a WM_GETOBJECT message carries when the caller wants the window's UI
	// Automation fragment root. Any other value is a request for a different accessibility interface (Microsoft Active
	// Accessibility, for instance) and must be refused so that the default window procedure can answer it.
	RootObjectId int32 = -25
	// AppendRuntimeId is the first element of a runtime identifier that is relative to the fragment root's own
	// runtime identifier, which UI Automation prepends. Every non-root element in a fragment uses it, so that runtime
	// identifiers stay unique across the windows of a process without the provider having to know the window's own
	// identifier.
	AppendRuntimeId int32 = 3
	// FrameworkID is the value this provider reports for FrameworkIdPropertyId, naming the toolkit that drew
	// the element. Clients use it only for diagnostics and for framework-specific workarounds.
	FrameworkID = "Unison"
)

// UI Automation HRESULT values. A provider returns one of these from a method it cannot satisfy; the client turns it
// back into the corresponding managed or scripting error.
//
// https://learn.microsoft.com/en-us/windows/win32/winauto/uiauto-error-codes
const (
	E_ELEMENTNOTENABLED   uint64 = 0x80040200
	E_ELEMENTNOTAVAILABLE uint64 = 0x80040201
	E_NOTSUPPORTED        uint64 = 0x80040204
	E_INVALIDOPERATION    uint64 = 0x80131509
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
