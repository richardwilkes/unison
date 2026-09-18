// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import "github.com/richardwilkes/unison/internal/dbus"

// The numbers in this file are part of the AT-SPI2 wire protocol rather than of any library, so they are transcribed
// from at-spi2-core's atspi-constants.h. Only the ones Unison actually reports are defined; the enumerations they come
// from have many more members, and the numbers of the ones left out are not reused.

// Role is the number that org.a11y.atspi.Accessible.GetRole reports, from AtspiRole.
type Role uint32

// The AT-SPI roles that Unison's own roles map onto. See [MapRole].
const (
	RoleInvalid       Role = 0   // ATSPI_ROLE_INVALID, which this package never reports; see [MapRole]
	RoleCheckBox      Role = 7   // ATSPI_ROLE_CHECK_BOX
	RoleCheckMenuItem Role = 8   // ATSPI_ROLE_CHECK_MENU_ITEM
	RoleColumnHeader  Role = 10  // ATSPI_ROLE_COLUMN_HEADER
	RoleComboBox      Role = 11  // ATSPI_ROLE_COMBO_BOX
	RoleDialog        Role = 16  // ATSPI_ROLE_DIALOG
	RoleFrame         Role = 23  // ATSPI_ROLE_FRAME
	RoleImage         Role = 27  // ATSPI_ROLE_IMAGE
	RoleLabel         Role = 29  // ATSPI_ROLE_LABEL
	RoleList          Role = 31  // ATSPI_ROLE_LIST
	RoleListItem      Role = 32  // ATSPI_ROLE_LIST_ITEM
	RoleMenu          Role = 33  // ATSPI_ROLE_MENU
	RoleMenuBar       Role = 34  // ATSPI_ROLE_MENU_BAR
	RoleMenuItem      Role = 35  // ATSPI_ROLE_MENU_ITEM
	RolePageTab       Role = 37  // ATSPI_ROLE_PAGE_TAB
	RolePageTabList   Role = 38  // ATSPI_ROLE_PAGE_TAB_LIST
	RolePanel         Role = 39  // ATSPI_ROLE_PANEL
	RolePasswordText  Role = 40  // ATSPI_ROLE_PASSWORD_TEXT
	RoleProgressBar   Role = 42  // ATSPI_ROLE_PROGRESS_BAR
	RolePushButton    Role = 43  // ATSPI_ROLE_PUSH_BUTTON
	RoleRadioButton   Role = 44  // ATSPI_ROLE_RADIO_BUTTON
	RoleScrollBar     Role = 48  // ATSPI_ROLE_SCROLL_BAR
	RoleScrollPane    Role = 49  // ATSPI_ROLE_SCROLL_PANE
	RoleSeparator     Role = 50  // ATSPI_ROLE_SEPARATOR
	RoleSlider        Role = 51  // ATSPI_ROLE_SLIDER
	RoleSpinButton    Role = 52  // ATSPI_ROLE_SPIN_BUTTON
	RoleTable         Role = 55  // ATSPI_ROLE_TABLE
	RoleTableCell     Role = 56  // ATSPI_ROLE_TABLE_CELL
	RoleText          Role = 61  // ATSPI_ROLE_TEXT
	RoleToggleButton  Role = 62  // ATSPI_ROLE_TOGGLE_BUTTON
	RoleToolBar       Role = 63  // ATSPI_ROLE_TOOL_BAR
	RoleToolTip       Role = 64  // ATSPI_ROLE_TOOL_TIP
	RoleTreeTable     Role = 66  // ATSPI_ROLE_TREE_TABLE
	RoleUnknown       Role = 67  // ATSPI_ROLE_UNKNOWN
	RoleParagraph     Role = 73  // ATSPI_ROLE_PARAGRAPH
	RoleApplication   Role = 75  // ATSPI_ROLE_APPLICATION
	RoleEntry         Role = 79  // ATSPI_ROLE_ENTRY
	RoleDocumentFrame Role = 82  // ATSPI_ROLE_DOCUMENT_FRAME
	RoleHeading       Role = 83  // ATSPI_ROLE_HEADING
	RoleLink          Role = 88  // ATSPI_ROLE_LINK
	RoleTableRow      Role = 90  // ATSPI_ROLE_TABLE_ROW
	RoleListBox       Role = 98  // ATSPI_ROLE_LIST_BOX
	RoleGrouping      Role = 99  // ATSPI_ROLE_GROUPING
	RoleBlockQuote    Role = 105 // ATSPI_ROLE_BLOCK_QUOTE
)

// StateBit is the position of one state within the bitset that org.a11y.atspi.Accessible.GetState reports, from
// AtspiStateType.
type StateBit uint32

// The AT-SPI states that Unison reports. See [States].
const (
	StateActive             StateBit = 1  // ATSPI_STATE_ACTIVE
	StateBusy               StateBit = 3  // ATSPI_STATE_BUSY
	StateChecked            StateBit = 4  // ATSPI_STATE_CHECKED
	StateCollapsed          StateBit = 5  // ATSPI_STATE_COLLAPSED
	StateEditable           StateBit = 7  // ATSPI_STATE_EDITABLE
	StateEnabled            StateBit = 8  // ATSPI_STATE_ENABLED
	StateExpandable         StateBit = 9  // ATSPI_STATE_EXPANDABLE
	StateExpanded           StateBit = 10 // ATSPI_STATE_EXPANDED
	StateFocusable          StateBit = 11 // ATSPI_STATE_FOCUSABLE
	StateFocused            StateBit = 12 // ATSPI_STATE_FOCUSED
	StateHorizontal         StateBit = 14 // ATSPI_STATE_HORIZONTAL
	StateModal              StateBit = 16 // ATSPI_STATE_MODAL
	StateMultiLine          StateBit = 17 // ATSPI_STATE_MULTI_LINE
	StateMultiselectable    StateBit = 18 // ATSPI_STATE_MULTISELECTABLE
	StatePressed            StateBit = 20 // ATSPI_STATE_PRESSED
	StateResizable          StateBit = 21 // ATSPI_STATE_RESIZABLE
	StateSelectable         StateBit = 22 // ATSPI_STATE_SELECTABLE
	StateSelected           StateBit = 23 // ATSPI_STATE_SELECTED
	StateSensitive          StateBit = 24 // ATSPI_STATE_SENSITIVE
	StateShowing            StateBit = 25 // ATSPI_STATE_SHOWING
	StateSingleLine         StateBit = 26 // ATSPI_STATE_SINGLE_LINE
	StateVertical           StateBit = 29 // ATSPI_STATE_VERTICAL
	StateVisible            StateBit = 30 // ATSPI_STATE_VISIBLE
	StateManagesDescendants StateBit = 31 // ATSPI_STATE_MANAGES_DESCENDANTS
	StateIndeterminate      StateBit = 32 // ATSPI_STATE_INDETERMINATE
	StateInvalidEntry       StateBit = 36 // ATSPI_STATE_INVALID_ENTRY
	StateSelectableText     StateBit = 38 // ATSPI_STATE_SELECTABLE_TEXT
	StateCheckable          StateBit = 41 // ATSPI_STATE_CHECKABLE
	StateHasPopup           StateBit = 42 // ATSPI_STATE_HAS_POPUP
	StateReadOnly           StateBit = 43 // ATSPI_STATE_READ_ONLY
)

// stateWords is how many 32-bit words a state bitset is made of. AT-SPI has always used two, and
// ATSPI_STATE_LAST_DEFINED is 44, so there is plenty of room left in the second one.
const stateWords = 2

// StateSet is the set of states of one object, held as the two 32-bit words that org.a11y.atspi.Accessible.GetState
// reports, least significant word first.
type StateSet [stateWords]uint32

// With returns the set with the given states added. A state outside the bitset, which AT-SPI has never defined, is
// ignored.
func (s StateSet) With(states ...StateBit) StateSet {
	for _, state := range states {
		mask := uint32(1) << (uint32(state) % 32)
		switch uint32(state) / 32 {
		case 0:
			s[0] |= mask
		case 1:
			s[1] |= mask
		default:
		}
	}
	return s
}

// Has returns true if the set contains the given state.
func (s StateSet) Has(state StateBit) bool {
	mask := uint32(1) << (uint32(state) % 32)
	switch uint32(state) / 32 {
	case 0:
		return s[0]&mask != 0
	case 1:
		return s[1]&mask != 0
	default:
		return false
	}
}

// Words returns the set as the words of the "au" reply that org.a11y.atspi.Accessible.GetState sends.
func (s StateSet) Words() []uint32 {
	return []uint32{s[0], s[1]}
}

// Relation is the number that identifies one kind of relation within the reply of
// org.a11y.atspi.Accessible.GetRelationSet, from AtspiRelationType.
type Relation uint32

// The AT-SPI relations that Unison reports. Each of them comes in a pair: the snapshot holds one direction and
// [buildRelations] derives the other.
const (
	RelationLabelFor       Relation = 1  // ATSPI_RELATION_LABEL_FOR
	RelationLabelledBy     Relation = 2  //nolint:misspell // ATSPI_RELATION_LABELLED_BY, AT-SPI's own spelling
	RelationControllerFor  Relation = 3  // ATSPI_RELATION_CONTROLLER_FOR
	RelationControlledBy   Relation = 4  // ATSPI_RELATION_CONTROLLED_BY
	RelationDescriptionFor Relation = 17 // ATSPI_RELATION_DESCRIPTION_FOR
	RelationDescribedBy    Relation = 18 // ATSPI_RELATION_DESCRIBED_BY
)

// CoordType is the coordinate space that a org.a11y.atspi.Component method works in, from AtspiCoordType. Every
// coordinate AT-SPI exchanges is in physical pixels.
type CoordType uint32

// The possible coordinate spaces.
const (
	// CoordScreen is relative to the top left corner of the screen.
	CoordScreen CoordType = 0
	// CoordWindow is relative to the top left corner of the window's content area.
	CoordWindow CoordType = 1
	// CoordParent is relative to the top left corner of the object's parent.
	CoordParent CoordType = 2
)

// Granularity is the unit that org.a11y.atspi.Text.GetStringAtOffset works in, from AtspiTextGranularity.
type Granularity uint32

// The granularities AT-SPI defines. All of them are answered; see [unitForGranularity].
const (
	GranularityChar      Granularity = 0 // ATSPI_TEXT_GRANULARITY_CHAR
	GranularityWord      Granularity = 1 // ATSPI_TEXT_GRANULARITY_WORD
	GranularitySentence  Granularity = 2 // ATSPI_TEXT_GRANULARITY_SENTENCE
	GranularityLine      Granularity = 3 // ATSPI_TEXT_GRANULARITY_LINE
	GranularityParagraph Granularity = 4 // ATSPI_TEXT_GRANULARITY_PARAGRAPH
)

// Boundary is the unit that org.a11y.atspi.Text.GetTextAtOffset, GetTextBeforeOffset and GetTextAfterOffset work in,
// from AtspiTextBoundaryType. It is a separate enumeration from [Granularity], with different numbers for the same
// units, because the three methods that take it predate GetStringAtOffset.
type Boundary uint32

// The boundary types AT-SPI defines. Each unit comes in a START and an END form, which differ in where the range a
// method answers with begins and ends relative to the unit; see [unitForBoundary].
const (
	BoundaryChar          Boundary = 0 // ATSPI_TEXT_BOUNDARY_CHAR
	BoundaryWordStart     Boundary = 1 // ATSPI_TEXT_BOUNDARY_WORD_START
	BoundaryWordEnd       Boundary = 2 // ATSPI_TEXT_BOUNDARY_WORD_END
	BoundarySentenceStart Boundary = 3 // ATSPI_TEXT_BOUNDARY_SENTENCE_START
	BoundarySentenceEnd   Boundary = 4 // ATSPI_TEXT_BOUNDARY_SENTENCE_END
	BoundaryLineStart     Boundary = 5 // ATSPI_TEXT_BOUNDARY_LINE_START
	BoundaryLineEnd       Boundary = 6 // ATSPI_TEXT_BOUNDARY_LINE_END
)

// Layer is the number that org.a11y.atspi.Component.GetLayer reports, from AtspiComponentLayer.
type Layer uint32

// The layers Unison reports.
const (
	// LayerWidget is the layer everything inside a window is in.
	LayerWidget Layer = 3
	// LayerWindow is the layer a top-level window itself is in.
	LayerWindow Layer = 7
)

// MatchType is how one criterion of an org.a11y.atspi.Collection match rule is compared against an object, from
// AtspiCollectionMatchType. A rule carries one per criterion — one for the states it names, one for the attributes, one
// for the roles and one for the interfaces — so a client can ask for any of several roles while insisting on all of
// several states.
type MatchType int32

// The ways a criterion is compared. See [matchRule.matches].
const (
	// MatchInvalid is the uninitialized value, which this package reads as a criterion that constrains nothing.
	MatchInvalid MatchType = 0 // ATSPI_Collection_MATCH_INVALID
	// MatchAll requires every member of the criterion to be present on the object.
	MatchAll MatchType = 1 // ATSPI_Collection_MATCH_ALL
	// MatchAny requires at least one member of the criterion to be present on the object.
	MatchAny MatchType = 2 // ATSPI_Collection_MATCH_ANY
	// MatchNone requires no member of the criterion to be present on the object.
	MatchNone MatchType = 3 // ATSPI_Collection_MATCH_NONE
	// MatchEmpty is MatchAll for a criterion that names anything at all, and requires the object's own set to be empty
	// for one that names nothing.
	MatchEmpty MatchType = 4 // ATSPI_Collection_MATCH_EMPTY
)

// SortOrder is the order an org.a11y.atspi.Collection search reports its matches in, from AtspiCollectionSortOrder.
type SortOrder uint32

// The orders AT-SPI defines. Only the canonical one and its reverse mean anything here: flow order and tab order are
// answered as the canonical order is, since a Unison window's reading order is the order its panels are described in
// and nothing separate says how the focus moves through it.
const (
	SortInvalid          SortOrder = 0 // ATSPI_Collection_SORT_ORDER_INVALID
	SortCanonical        SortOrder = 1 // ATSPI_Collection_SORT_ORDER_CANONICAL
	SortFlow             SortOrder = 2 // ATSPI_Collection_SORT_ORDER_FLOW
	SortTab              SortOrder = 3 // ATSPI_Collection_SORT_ORDER_TAB
	SortReverseCanonical SortOrder = 4 // ATSPI_Collection_SORT_ORDER_REVERSE_CANONICAL
	SortReverseFlow      SortOrder = 5 // ATSPI_Collection_SORT_ORDER_REVERSE_FLOW
	SortReverseTab       SortOrder = 6 // ATSPI_Collection_SORT_ORDER_REVERSE_TAB
)

// TreeTraversal is how much of the hierarchy around the object a search starts at an org.a11y.atspi.Collection search
// considers, from AtspiCollectionTreeTraversalType. It only applies to the two searches that start somewhere within the
// collection; see [nodeObject.getMatchesFrom].
type TreeTraversal uint32

// The traversals AT-SPI defines.
const (
	// TreeRestrictChildren looks only within the object the search starts at.
	TreeRestrictChildren TreeTraversal = 0 // ATSPI_Collection_TREE_RESTRICT_CHILDREN
	// TreeRestrictSibling looks only at the objects that sit beside the object the search starts at.
	TreeRestrictSibling TreeTraversal = 1 // ATSPI_Collection_TREE_RESTRICT_SIBLING
	// TreeInorder looks at everything the collection holds, in reading order, which is what Orca asks for.
	TreeInorder TreeTraversal = 2 // ATSPI_Collection_TREE_INORDER
)

// The AT-SPI interface names.
const (
	// InterfaceAccessible is implemented by every object.
	InterfaceAccessible = "org.a11y.atspi.Accessible"
	// InterfaceAction is implemented by the objects that can be asked to do something.
	InterfaceAction = "org.a11y.atspi.Action"
	// InterfaceApplication is implemented by the application root.
	InterfaceApplication = "org.a11y.atspi.Application"
	// InterfaceCache is implemented by the cache object.
	InterfaceCache = "org.a11y.atspi.Cache"
	// InterfaceCollection is implemented by every object, and is how an assistive technology finds the objects within a
	// subtree that match a description instead of walking it a call at a time. Orca's structural navigation — the
	// commands that jump by heading, link, list, table or paragraph — is built on it alone, and announces "not
	// supported" for a document whose objects do not implement it.
	InterfaceCollection = "org.a11y.atspi.Collection"
	// InterfaceComponent is implemented by every object that occupies space on the screen.
	InterfaceComponent = "org.a11y.atspi.Component"
	// InterfaceEditableText is implemented by the objects whose text can be changed, which is the only way AT-SPI has
	// of typing into a control.
	InterfaceEditableText = "org.a11y.atspi.EditableText"
	// InterfaceHyperlink is implemented by the objects that lead somewhere, which is what a link does, and by the
	// objects that occupy part of another object's text, which is what a link or an image within a paragraph does. It is
	// the object itself that implements it rather than a separate one: libatspi's atspi_accessible_get_hyperlink hands
	// back a hyperlink at the very same path.
	InterfaceHyperlink = "org.a11y.atspi.Hyperlink"
	// InterfaceHypertext is implemented by the objects whose text holds such objects, and is how an assistive technology
	// asks which of them a paragraph holds and where in its text each one sits.
	InterfaceHypertext = "org.a11y.atspi.Hypertext"
	// InterfaceSelection is implemented by the containers whose children can be selected.
	InterfaceSelection = "org.a11y.atspi.Selection"
	// InterfaceSocket is what the registry implements, and is only ever called rather than answered.
	InterfaceSocket = "org.a11y.atspi.Socket"
	// InterfaceTable is implemented by the containers laid out as a grid of cells, which is what an assistive
	// technology's table navigation commands work over.
	InterfaceTable = "org.a11y.atspi.Table"
	// InterfaceTableCell is implemented by the cells of such a container, and is how one says where in the grid it
	// sits.
	InterfaceTableCell = "org.a11y.atspi.TableCell"
	// InterfaceText is implemented by the objects that hold navigable text.
	InterfaceText = "org.a11y.atspi.Text"
	// InterfaceValue is implemented by the objects that hold a numeric value.
	InterfaceValue = "org.a11y.atspi.Value"
)

// The AT-SPI event interface names. An AT-SPI event is named class:major:minor, and the class is what becomes the D-Bus
// interface of the signal, so these are the classes of the events Unison sends. Nothing implements them: they exist
// only as the interface an event carries, which is what an assistive technology matches its listeners against.
const (
	// InterfaceEventObject is the class of the events about an object itself, which is most of them.
	InterfaceEventObject = "org.a11y.atspi.Event.Object"
	// InterfaceEventWindow is the class of the events about a top-level window's lifetime.
	InterfaceEventWindow = "org.a11y.atspi.Event.Window"
	// InterfaceEventFocus is the class of the one legacy event that says where the keyboard focus has gone. It predates
	// the FOCUSED state change that says the same thing, and assistive technologies still listen for both.
	InterfaceEventFocus = "org.a11y.atspi.Event.Focus"
)

// The object paths of the AT-SPI objects.
const (
	// RootPath is where the object representing the application as a whole is exported.
	RootPath dbus.ObjectPath = "/org/a11y/atspi/accessible/root"
	// CachePath is where the cache object is exported.
	CachePath dbus.ObjectPath = "/org/a11y/atspi/cache"
	// AccessiblePrefix is the subtree the per-node objects are exported below.
	AccessiblePrefix dbus.ObjectPath = "/org/a11y/atspi/accessible"
	// NullPath is the path of the null object reference, which is how AT-SPI says "nothing".
	NullPath dbus.ObjectPath = "/org/a11y/atspi/null"
)

// The peers this package talks to.
const (
	// RegistryDestination is the bus name of the AT-SPI registry daemon, which is called to join the accessibility
	// tree and told when the application leaves it again.
	RegistryDestination = "org.a11y.atspi.Registry"
	// BusDestination is the bus name, on the session bus, of the accessibility bus launcher.
	BusDestination = "org.a11y.Bus"
	// BusPath is the path of the launcher's object.
	BusPath dbus.ObjectPath = "/org/a11y/bus"
	// BusInterface is the launcher's own interface, which hands out the address of the accessibility bus.
	BusInterface = "org.a11y.Bus"
	// StatusInterface is the launcher interface whose IsEnabled property says whether an assistive technology is
	// running.
	StatusInterface = "org.a11y.Status"
)

// The signatures of the AT-SPI replies, spelled out once each so that the object definitions read as the interface
// specifications do.
const (
	objectRefSignature      dbus.Signature = "(so)"
	objectRefArraySignature dbus.Signature = "a(so)"
	relationSetSignature    dbus.Signature = "a(ua(so))"
	stringDictSignature     dbus.Signature = "a{ss}"
	stateSignature          dbus.Signature = "au"
	extentsSignature        dbus.Signature = "(iiii)"
	// intPairAndCoordSignature is the arguments of the methods that take two integers and a coordinate type: a point,
	// for the component ones, or a range of character offsets, for the text ones.
	intPairAndCoordSignature dbus.Signature = "iiu"
	cacheItemSignature       dbus.Signature = "((so)(so)(so)iiassusau)"
	cacheItemsSignature      dbus.Signature = "a((so)(so)(so)iiassusau)"
	textRangeSignature       dbus.Signature = "sii"
	textExtentsSignature     dbus.Signature = "iiii"
	textAttributesSignature  dbus.Signature = "a{ss}ii"
	intPairSignature         dbus.Signature = "(ii)"
	int32ArraySignature      dbus.Signature = "ai"
	// rowColumnExtentsSignature is the reply of org.a11y.atspi.Table.GetRowColumnExtentsAtIndex: whether there is a
	// cell there at all, its row and column, how many of each it spans, and whether it is selected.
	rowColumnExtentsSignature dbus.Signature = "biiiib"
	// rowColumnSpanSignature is the reply of org.a11y.atspi.TableCell.GetRowColumnSpan, which is the same without the
	// selection.
	rowColumnSpanSignature dbus.Signature = "biiii"
	// matchRuleSignature is the description of the objects an org.a11y.atspi.Collection search is looking for: the
	// states, the attributes, the roles and the interfaces it asks about, each followed by the [MatchType] saying how
	// that criterion is to be compared, and finally whether the whole rule is inverted. The states and the roles arrive
	// as bit arrays of 32-bit words rather than as lists of numbers; see [decodeMatchRule].
	matchRuleSignature dbus.Signature = "(aiia{ss}iaiiasib)"
	// eventSignature is the body of every AT-SPI event: the detail string, the two integers whose meaning depends on
	// the event, the value it carries, and the properties of the object it came from. libatspi refuses an event with
	// any other signature, so this is the one place it is spelled out.
	eventSignature dbus.Signature = "siiva{sv}"
)

// The standard D-Bus interfaces this package needs by name, which the dbus package answers for every object it exports
// but does not make public.
const (
	dbusInterface           = "org.freedesktop.DBus"
	dbusPropertiesInterface = "org.freedesktop.DBus.Properties"
	// dbusDestination is the bus's own name, which is also the sender of every signal the bus itself emits, such as
	// NameOwnerChanged. A match rule that names it, and a check of the sender when one arrives, are what keep another
	// peer on the same bus from forging one.
	dbusDestination = "org.freedesktop.DBus"
	// dbusObjectPath is the path of the bus's own object, which those signals come from.
	dbusObjectPath dbus.ObjectPath = "/org/freedesktop/DBus"
	// uniqueNamePrefix begins every name the bus hands out to a connection of its own accord, which is how one is told
	// apart from a well-known name a connection asked for and from the bus's own name.
	uniqueNamePrefix = ":"
)

const (
	// toolkitName is what org.a11y.atspi.Application.ToolkitName reports, and also the value of the "toolkit"
	// attribute of every object.
	toolkitName = "unison"
	// atspiVersion is the version of AT-SPI this server speaks. 2.1 is what every current toolkit reports, since the
	// interfaces have not changed incompatibly since.
	atspiVersion = "2.1"
)
