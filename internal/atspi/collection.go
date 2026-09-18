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
	"math/bits"
	"slices"
	"strings"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// matchRuleFields is how many fields a match rule holds; see [matchRuleSignature].
const matchRuleFields = 9

// collectionInterface returns the org.a11y.atspi.Collection interface of a node, which hands an assistive technology
// every object within this one that matches a description in a single call.
//
// It is what a screen reader's structural navigation is built on. Orca's H, K, L, T and P commands — jump to the next
// heading, link, list, table or paragraph — ask the object the focus is inside for the matches of a rule and move to
// the first one; with no Collection interface to ask, Orca announces that structural navigation is not supported and
// the document cannot be navigated at all. Every object implements it, as at-spi2-atk's does, since a client may search
// from whichever object it happens to be holding; a search from a leaf simply finds nothing.
//
// Everything is answered from the published snapshot, so a search costs one walk of the subtree with no round trips.
// GetActiveDescendant is answered with the null reference; see [nodeObject.getActiveDescendant].
func (o *nodeObject) collectionInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceCollection,
		Methods: []*dbus.Method{
			{Name: "GetMatches", In: matchRuleSignature + "uib", Out: objectRefArraySignature, Handle: o.getMatches},
			{
				Name: "GetMatchesTo", In: objectRefSignature + matchRuleSignature + "uubib",
				Out: objectRefArraySignature, Handle: o.getMatchesTo,
			},
			{
				Name: "GetMatchesFrom", In: objectRefSignature + matchRuleSignature + "uuib",
				Out: objectRefArraySignature, Handle: o.getMatchesFrom,
			},
			{Name: "GetActiveDescendant", Out: objectRefSignature, Handle: o.getActiveDescendant},
		},
	}
}

// getMatches implements org.a11y.atspi.Collection.GetMatches, which searches the whole of this object for the
// descendants that satisfy a rule. count is the most matches to hand back, with anything at or below zero meaning all
// of them, and traverse says whether to look below the object's own children.
func (o *nodeObject) getMatches(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	rule, failure := decodeMatchRule(argAt(args, 0))
	if failure != nil {
		call.Error(failure.Name, failure.Message)
		return
	}
	matches := o.matchesOf(rule, SortOrder(uint32Arg(args, 1)), int(int32Arg(args, 2)),
		o.candidateWalk(boolArg(args, 3)))
	call.Reply(o.a.references(matches))
}

// getMatchesFrom implements org.a11y.atspi.Collection.GetMatchesFrom, which searches the part of this object that comes
// after another object within it: "the next heading", asked from the document and told where the reader is. The matches
// are handed back nearest first, since a client asking for one of them wants the one it is about to move to; Orca's
// forward structural navigation asks for exactly one.
func (o *nodeObject) getMatchesFrom(call *dbus.Call) {
	o.replyWithRelativeMatches(call, false)
}

// getMatchesTo implements org.a11y.atspi.Collection.GetMatchesTo, which is the same search backwards: the matches
// within this object that come before another object within it, nearest first, which is what Orca's backward structural
// navigation moves to.
//
// Its one extra argument, limitScope, keeps the search within the reported parent of the object it starts at rather
// than letting it reach the whole of the collection.
func (o *nodeObject) getMatchesTo(call *dbus.Call) {
	o.replyWithRelativeMatches(call, true)
}

// replyWithRelativeMatches answers the two searches that start somewhere within this object. backwards says which of
// them it is, since the two differ only in the direction they look in and in the one extra argument GetMatchesTo
// carries; see [relativeSearch].
func (o *nodeObject) replyWithRelativeMatches(call *dbus.Call, backwards bool) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	rule, failure := decodeMatchRule(argAt(args, 1))
	if failure != nil {
		call.Error(failure.Name, failure.Message)
		return
	}
	search := relativeSearch{
		current:   o.nodeArg(args, 0),
		order:     SortOrder(uint32Arg(args, 2)),
		tree:      TreeTraversal(uint32Arg(args, 3)),
		backwards: backwards,
	}
	// GetMatchesTo has limitScope between the traversal and the count, which is the only difference between the two
	// argument lists.
	next := 4
	if backwards {
		search.limitScope = boolArg(args, next)
		next++
	}
	search.count = int(int32Arg(args, next))
	search.traverse = boolArg(args, next+1)
	call.Reply(o.a.references(o.matchesRelativeTo(rule, search)))
}

// getActiveDescendant implements org.a11y.atspi.Collection.GetActiveDescendant, which asks a container that manages its
// own descendants which of them is the current one. Nothing here has one to report: the containers that make that
// promise — a table or a tree large enough for [ManagesDescendants] — announce the current row through
// object:active-descendant-changed as it moves, which is what an assistive technology honoring that state follows, and
// the null reference is how AT-SPI says there is nothing here.
func (o *nodeObject) getActiveDescendant(call *dbus.Call) {
	call.Reply(nullReference())
}

// relativeSearch is what one of the two searches that start somewhere within the collection was asked for.
type relativeSearch struct {
	// current is the node the search starts at, or zero when the reference named nothing this window holds.
	current accessibility.NodeID
	order   SortOrder
	tree    TreeTraversal
	count   int
	// traverse says whether the search looks below the collection's own children.
	traverse bool
	// limitScope keeps a backward search within the reported parent of the node it starts at. Only GetMatchesTo carries
	// it.
	limitScope bool
	// backwards says the search looks before the node it starts at rather than after it.
	backwards bool
}

// candidates is a walk over the nodes a search considers, in the order an assistive technology reads them, handing each
// of them to fn and stopping as soon as fn returns false. Nothing is materialized: a search that has found everything
// it was asked for leaves the rest of the document unvisited, which is what a walk rather than a slice is for; see
// [nodeObject.matchesOf].
type candidates func(fn func(n *accessibility.Node) bool)

// candidateWalk returns the walk over the nodes a search over this object considers: every reported descendant in
// pre-order, or only the reported children when the caller did not ask to traverse. The object itself is never one of
// them, which is what at-spi2-atk does as well: a client searching from the document for the next heading must not be
// handed the document.
func (o *nodeObject) candidateWalk(traverse bool) candidates {
	if !traverse {
		return func(fn func(n *accessibility.Node) bool) {
			for _, id := range o.children() {
				if n := o.data.node(id); n != nil && !fn(n) {
					return
				}
			}
		}
	}
	return func(fn func(n *accessibility.Node) bool) { o.data.walkReported(o.node.ID, fn) }
}

// candidateWalkOver returns the walk over a list of nodes that has already been worked out, which is what a backward
// search hands its reversed run of candidates over as.
func (o *nodeObject) candidateWalkOver(ids []accessibility.NodeID) candidates {
	return func(fn func(n *accessibility.Node) bool) {
		for _, id := range ids {
			if n := o.data.node(id); n != nil && !fn(n) {
				return
			}
		}
	}
}

// matchesOf returns the candidates whose objects satisfy the rule, in the order the caller asked for and cut down to
// the number it asked for.
//
// The walk stops as soon as the search has what it asked for, which is what keeps a forward structural-navigation
// keypress from costing the rest of the document: every press of Orca's H, K, L, T or P asks for exactly one match from
// where the reader is, and the rule that answers it needs to be tried only as far as the object the reader is about to
// move to. A candidate hands its match back before the walk decides whether to go on, so the answer is the same
// whichever way it was arrived at.
//
// A reversed order is the one that cannot stop. The match it hands back first is the one farthest from the starting
// point, so every candidate has to be seen before the first of them is known; see [limitMatches], which is what puts
// the matches into that order.
func (o *nodeObject) matchesOf(rule *matchRule, order SortOrder, count int, walk candidates) []accessibility.NodeID {
	enough := count
	if reversedOrder(order) {
		enough = 0
	}
	matches := make([]accessibility.NodeID, 0, 8)
	walk(func(n *accessibility.Node) bool {
		if n.Ignored || !rule.matches(o.data, n) {
			return true
		}
		matches = append(matches, n.ID)
		return enough <= 0 || len(matches) < enough
	})
	return limitMatches(matches, order, count)
}

// startingAfter returns the walk over the candidates that come after one of them, which is where a forward search
// begins. A start the walk never reaches leaves nothing at all: the client is holding a reference from a snapshot that
// has been replaced, and will ask again about the object it is given instead. reached says that the start has already
// been passed before the walk begins, which is what the collection itself is, since everything within it lies after it.
func startingAfter(walk candidates, start accessibility.NodeID, reached bool) candidates {
	if reached {
		return walk
	}
	return func(fn func(n *accessibility.Node) bool) {
		passed := false
		walk(func(n *accessibility.Node) bool {
			if !passed {
				passed = n.ID == start
				return true
			}
			return fn(n)
		})
	}
}

// matchesRelativeTo returns the matches on one side of the node a search starts at, nearest first, so that a client
// asking for a single match is handed the one it is about to move to. A reversed sort order turns that around and hands
// back the match farthest from the starting point first.
//
// A start that this object does not hold has no place in its order, so nothing is found: the client is holding a
// reference from a snapshot that has been replaced, and will ask again about the object it is given instead. The
// collection itself is the one exception, since everything within it lies after it.
//
// Only a forward search can be walked as it goes. A backward one hands back the candidate nearest the start first,
// which is the order the run before the start reversed, and that run is not known until the walk has reached the start;
// see [nodeObject.candidatesBefore].
func (o *nodeObject) matchesRelativeTo(rule *matchRule, search relativeSearch) []accessibility.NodeID {
	walk := o.candidatesBefore(search)
	if !search.backwards {
		walk = startingAfter(o.candidateWalk(search.traverse), search.current, search.current == o.node.ID)
	}
	return o.matchesOf(rule, search.order, search.count, o.inSearchScope(walk, search))
}

// candidatesBefore returns the walk over the candidates that come before the node a backward search starts at, nearest
// first, which is the run up to that node reversed. A backward search from the collection itself has nothing before it,
// and neither has one from a node that is none of this search's candidates: the walk that would have reached it ran out
// first.
func (o *nodeObject) candidatesBefore(search relativeSearch) candidates {
	if search.current == o.node.ID {
		return noCandidates
	}
	ids := make([]accessibility.NodeID, 0, 32)
	reached := false
	o.candidateWalk(search.traverse)(func(n *accessibility.Node) bool {
		if n.ID == search.current {
			reached = true
			return false
		}
		ids = append(ids, n.ID)
		return true
	})
	if !reached {
		return noCandidates
	}
	slices.Reverse(ids)
	return o.candidateWalkOver(ids)
}

// noCandidates is the walk over nothing at all, which is what a search that cannot find anything wherever it looks is
// given rather than a special case of its own.
func noCandidates(_ func(n *accessibility.Node) bool) {}

// inSearchScope returns the walk confined to the candidates that the traversal the caller asked for, and the scope
// limit a backward search may carry, leave in play. Orca asks for the whole collection in reading order, which is what
// [TreeInorder] means and what leaves every candidate here; the other two traversals confine the search to the object
// it starts at, or to that object's siblings, and limitScope confines it to that object's reported parent.
func (o *nodeObject) inSearchScope(walk candidates, search relativeSearch) candidates {
	within := o.searchScopeMembers(search)
	if within == nil {
		return walk
	}
	return func(fn func(n *accessibility.Node) bool) {
		walk(func(n *accessibility.Node) bool {
			if !within[n.ID] {
				return true
			}
			return fn(n)
		})
	}
}

// searchScopeMembers returns the nodes a search is confined to, or nil when it is confined to nothing but the
// collection it was asked of. A traversal that confines the search already says more than the scope limit does, so the
// limit only applies to a search over the whole collection.
func (o *nodeObject) searchScopeMembers(search relativeSearch) map[accessibility.NodeID]bool {
	switch search.tree {
	case TreeRestrictChildren:
		return o.data.reportedDescendants(search.current)
	case TreeRestrictSibling:
		siblings := make(map[accessibility.NodeID]bool)
		for _, id := range o.data.unignoredChildren(o.data.parent(search.current)) {
			siblings[id] = true
		}
		return siblings
	default: // TreeInorder, along with anything AT-SPI has not defined
		if search.limitScope {
			return o.data.reportedDescendants(o.data.parent(search.current))
		}
		return nil
	}
}

// reportedDescendants returns the reported descendants of a node as a set, which is how the searches that are confined
// to part of a window ask whether a candidate is inside it.
func (d *windowData) reportedDescendants(id accessibility.NodeID) map[accessibility.NodeID]bool {
	within := make(map[accessibility.NodeID]bool)
	d.walkReported(id, func(n *accessibility.Node) bool {
		within[n.ID] = true
		return true
	})
	return within
}

// limitMatches puts the matches into the order the caller asked for and cuts them down to the number it asked for. A
// count at or below zero is how AT-SPI asks for all of them.
func limitMatches(matches []accessibility.NodeID, order SortOrder, count int) []accessibility.NodeID {
	if reversedOrder(order) {
		slices.Reverse(matches)
	}
	if count > 0 && len(matches) > count {
		matches = matches[:count]
	}
	return matches
}

// reversedOrder reports whether a sort order asks for the matches last first. The three orders that are not reversed,
// along with anything AT-SPI has not defined, hand them back in the order they were found in; see [SortOrder].
func reversedOrder(order SortOrder) bool {
	switch order {
	case SortReverseCanonical, SortReverseFlow, SortReverseTab:
		return true
	default:
		return false
	}
}

// matchRule is the description of the objects an org.a11y.atspi.Collection search is looking for, as [decodeMatchRule]
// reads it off the wire.
type matchRule struct {
	attributes      []attributeCriterion
	interfaces      []string
	roles           []uint32
	states          StateSet
	statesNamed     int
	rolesNamed      int
	statesMatch     MatchType
	attributesMatch MatchType
	rolesMatch      MatchType
	interfacesMatch MatchType
	invert          bool
}

// attributeCriterion is one object attribute a rule asks about, together with the values that satisfy it.
type attributeCriterion struct {
	key    string
	values []string
}

// matches reports whether a node is one of the objects the rule describes. Every criterion the rule names has to be
// satisfied, each in the way its own [MatchType] says; an inverted rule matches everything the rule as written does
// not.
//
// The criteria are tried cheapest first. The role is a switch on the node, the states a pair of words, while the
// attributes and the interfaces each build a list, so a rule that names a role — which is what every one of Orca's
// structural navigation rules does — rejects almost every candidate without allocating anything at all.
func (r *matchRule) matches(d *windowData, n *accessibility.Node) bool {
	matched := r.matchesRoles(n) && r.matchesStates(d, n) && r.matchesAttributes(d, n) && r.matchesInterfaces(d, n)
	return matched != r.invert
}

// matchesRoles answers the rule's role criterion. An object has exactly one role, so its own set is never empty and
// MatchAll can only ever be satisfied by a rule naming that one role; both fall out of [matchCriterion].
func (r *matchRule) matchesRoles(n *accessibility.Node) bool {
	if r.rolesNamed == 0 && r.rolesMatch != MatchEmpty {
		return true
	}
	present := 0
	if roleNamedIn(r.roles, MapRole(n)) {
		present = 1
	}
	return matchCriterion(r.rolesMatch, r.rolesNamed, present, false)
}

// matchesStates answers the rule's state criterion, against the very state set the object's own
// org.a11y.atspi.Accessible.GetState reports, so that a rule asking for what a client has just been told is there finds
// it.
func (r *matchRule) matchesStates(d *windowData, n *accessibility.Node) bool {
	if r.statesNamed == 0 && r.statesMatch != MatchEmpty {
		return true
	}
	set := States(n, d.active(), n.ID == d.tree.Root)
	return matchCriterion(r.statesMatch, r.statesNamed, statesPresent(r.states, set), statesNamed(set) == 0)
}

// matchesAttributes answers the rule's attribute criterion. Each attribute the rule names is satisfied when the object
// reports that attribute with one of the values the rule accepts, which is how Orca asks for a heading of a particular
// level, or for the code blocks among the paragraphs.
func (r *matchRule) matchesAttributes(d *windowData, n *accessibility.Node) bool {
	if len(r.attributes) == 0 && r.attributesMatch != MatchEmpty {
		return true
	}
	attributes := d.attributesOf(n)
	present := 0
	for _, criterion := range r.attributes {
		if criterion.satisfiedBy(attributes) {
			present++
		}
	}
	return matchCriterion(r.attributesMatch, len(r.attributes), present, len(attributes) == 0)
}

// matchesInterfaces answers the rule's interface criterion. Every object implements org.a11y.atspi.Accessible, so its
// own set is never empty.
func (r *matchRule) matchesInterfaces(d *windowData, n *accessibility.Node) bool {
	if len(r.interfaces) == 0 && r.interfacesMatch != MatchEmpty {
		return true
	}
	implemented := Interfaces(n, d.isSpanTarget(n.ID))
	present := 0
	for _, wanted := range r.interfaces {
		if interfaceNamedIn(implemented, wanted) {
			present++
		}
	}
	return matchCriterion(r.interfacesMatch, len(r.interfaces), present, false)
}

// satisfiedBy reports whether an object's attributes hold this criterion's attribute with one of the values it accepts.
func (c *attributeCriterion) satisfiedBy(attributes dbus.Dict) bool {
	for _, entry := range attributes {
		key, ok := entry.Key.(string)
		if !ok || key != c.key {
			continue
		}
		value, ok := entry.Value.(string)
		return ok && slices.Contains(c.values, value)
	}
	return false
}

// matchCriterion answers one criterion of a rule from how many members it names, how many of them the object has, and
// whether the object's own set of that kind is empty. The four ways of comparing are the ones AtspiCollectionMatchType
// defines, and an empty criterion constrains nothing in three of them: a rule that names no states is not a rule that
// only stateless objects satisfy, which is what at-spi2-atk answers as well. MatchEmpty is the exception, being there
// precisely to ask for an object with nothing of that kind.
//
// A criterion whose comparison is MatchInvalid is treated as naming nothing at all. AT-SPI's own clients always fill
// the four in, so an unset one is a client that has built a rule badly, and answering with no matches would leave a
// screen reader reporting that there is nothing in the document rather than that it asked wrongly.
func matchCriterion(kind MatchType, named, present int, objectEmpty bool) bool {
	switch kind {
	case MatchEmpty:
		if named == 0 {
			return objectEmpty
		}
		return present == named
	case MatchAll:
		return named == 0 || present == named
	case MatchAny:
		return named == 0 || present > 0
	case MatchNone:
		return present == 0
	default: // MatchInvalid
		return true
	}
}

// statesNamed returns how many states a set holds.
func statesNamed(set StateSet) int {
	total := 0
	for _, word := range set {
		total += bits.OnesCount32(word)
	}
	return total
}

// statesPresent returns how many of the states a rule names are in an object's set.
func statesPresent(named, set StateSet) int {
	total := 0
	for i := range named {
		total += bits.OnesCount32(named[i] & set[i])
	}
	return total
}

// roleNamedIn reports whether a rule's roles hold one. The roles arrive as a bit array rather than as a list of
// numbers: role r is bit r%32 of word r/32, least significant word first, which is what libatspi packs them into.
func roleNamedIn(words []uint32, r Role) bool {
	index := int(r) / 32
	if index < 0 || index >= len(words) {
		return false
	}
	return words[index]&(uint32(1)<<(uint32(r)%32)) != 0
}

// interfaceNamedIn reports whether a list of AT-SPI interface names holds one, comparing only the part after the last
// dot and ignoring case. A client may ask for "Text", for "text" or for the whole of "org.a11y.atspi.Text" — libatspi's
// own clients use all three — and at-spi2-atk matches them the same way.
func interfaceNamedIn(implemented []string, wanted string) bool {
	name := interfaceTail(wanted)
	for _, one := range implemented {
		if strings.EqualFold(interfaceTail(one), name) {
			return true
		}
	}
	return false
}

// interfaceTail returns the part of an interface name after the last dot, which is the whole of a name that has none.
func interfaceTail(name string) string {
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		return name[index+1:]
	}
	return name
}

// decodeMatchRule returns the rule one of the searching methods was handed. Everything about it is checked, since a
// rule is the one argument AT-SPI carries as a structure of nine fields and a client that has built one wrongly is
// better told so than answered with the matches of a rule it did not mean; see [matchRuleSignature].
func decodeMatchRule(value any) (*matchRule, *dbus.Error) {
	fields, ok := value.(dbus.Struct)
	if !ok || len(fields) != matchRuleFields {
		return nil, dbus.Errorf(dbus.InvalidArgs, "a match rule is a structure of %d fields", matchRuleFields)
	}
	states, failure := wordsField(fields[0], "states")
	if failure != nil {
		return nil, failure
	}
	roles, failure := wordsField(fields[4], "roles")
	if failure != nil {
		return nil, failure
	}
	attributes, failure := attributeCriteriaField(fields[2])
	if failure != nil {
		return nil, failure
	}
	interfaces, ok := fields[6].([]string)
	if !ok {
		return nil, dbus.Errorf(dbus.InvalidArgs, "the interfaces of a match rule are an array of strings")
	}
	invert, ok := fields[8].(bool)
	if !ok {
		return nil, dbus.Errorf(dbus.InvalidArgs, "whether a match rule is inverted is a boolean")
	}
	rule := &matchRule{
		attributes: attributes,
		interfaces: interfaces,
		roles:      roles,
		states:     stateSetOf(states),
		invert:     invert,
	}
	for _, one := range []struct {
		into  *MatchType
		field int
	}{
		{into: &rule.statesMatch, field: 1},
		{into: &rule.attributesMatch, field: 3},
		{into: &rule.rolesMatch, field: 5},
		{into: &rule.interfacesMatch, field: 7},
	} {
		if *one.into, failure = matchTypeField(fields[one.field]); failure != nil {
			return nil, failure
		}
	}
	rule.statesNamed = statesNamed(rule.states)
	for _, word := range rule.roles {
		rule.rolesNamed += bits.OnesCount32(word)
	}
	return rule, nil
}

// wordsField returns one of the two bit arrays of a match rule. They arrive as signed 32-bit integers, which is how
// AT-SPI carries a bitset of any size, and are read as the unsigned words they are.
func wordsField(value any, name string) ([]uint32, *dbus.Error) {
	numbers, ok := value.([]int32)
	if !ok {
		return nil, dbus.Errorf(dbus.InvalidArgs, "the %s of a match rule are an array of integers", name)
	}
	words := make([]uint32, 0, len(numbers))
	for _, number := range numbers {
		words = append(words, uint32(number))
	}
	return words, nil
}

// stateSetOf returns the state set a rule's words describe. AT-SPI has always carried a state set as two words and
// ATSPI_STATE_LAST_DEFINED is far short of filling them, so a client that sent more is answered from the two that mean
// something and one that sent fewer from what it did send.
func stateSetOf(words []uint32) StateSet {
	var set StateSet
	for i := range min(len(words), stateWords) {
		set[i] = words[i]
	}
	return set
}

// matchTypeField returns one of the four ways a rule compares a criterion. An unrecognized one is refused rather than
// guessed at: it is the field that decides whether a rule means "any of these" or "none of these", and the two answers
// have nothing in common.
func matchTypeField(value any) (MatchType, *dbus.Error) {
	number, ok := value.(int32)
	if !ok {
		return MatchInvalid, dbus.Errorf(dbus.InvalidArgs,
			"the way a match rule compares a criterion is an integer")
	}
	if number < int32(MatchInvalid) || number > int32(MatchEmpty) {
		return MatchInvalid, dbus.Errorf(dbus.InvalidArgs,
			"%d is not a way of comparing a criterion of a match rule", number)
	}
	return MatchType(number), nil
}

// attributeCriteriaField returns the attributes a match rule asks about, in the order the client wrote them.
func attributeCriteriaField(value any) ([]attributeCriterion, *dbus.Error) {
	dict, ok := value.(dbus.Dict)
	if !ok {
		return nil, dbus.Errorf(dbus.InvalidArgs, "the attributes of a match rule are a dictionary of strings")
	}
	criteria := make([]attributeCriterion, 0, len(dict))
	for _, entry := range dict {
		key, isString := entry.Key.(string)
		if !isString {
			return nil, dbus.Errorf(dbus.InvalidArgs, "the name of an attribute of a match rule is a string")
		}
		text, isString := entry.Value.(string)
		if !isString {
			return nil, dbus.Errorf(dbus.InvalidArgs, "the value of an attribute of a match rule is a string")
		}
		criteria = append(criteria, attributeCriterion{key: key, values: splitAttributeValues(text)})
	}
	return criteria, nil
}

// splitAttributeValues returns the values one attribute of a match rule accepts. Several are written as one string
// separated by colons, which is how libatspi's own rule builder packs the alternatives for an attribute; a colon that
// belongs to a value, and a backslash, are escaped with a backslash. A trailing backslash stands for itself, since
// there is nothing after it for it to escape.
func splitAttributeValues(text string) []string {
	values := make([]string, 0, 2)
	var current strings.Builder
	escaped := false
	for _, ch := range text {
		switch {
		case escaped:
			current.WriteRune(ch)
			escaped = false
		case ch == '\\':
			escaped = true
		case ch == ':':
			values = append(values, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if escaped {
		current.WriteByte('\\')
	}
	return append(values, current.String())
}
