// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package secrets

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"errors"
	"log/slog"
	"maps"
	"math/big"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/internal/dbus"
	"github.com/richardwilkes/unison/internal/testenv"
)

// ssTestTimeout bounds every wait in these tests, so that a bug fails a test rather than hanging the suite. It only
// decides how long a failure takes to report.
const ssTestTimeout = 10 * time.Second

const (
	// fakeServiceOwner is the unique name the bus would stamp on the fake service's replies and signals.
	fakeServiceOwner = ":1.5"
	// fakeOtherSender is another connection on the same bus.
	fakeOtherSender                    = ":1.99"
	fakeBusName                        = "org.freedesktop.DBus"
	fakeBusPath        dbus.ObjectPath = "/org/freedesktop/DBus"
	fakeLogin          dbus.ObjectPath = "/org/freedesktop/secrets/collection/login"
	fakeWork           dbus.ObjectPath = "/org/freedesktop/secrets/collection/work"
	fakeDefaultKeyring dbus.ObjectPath = "/org/freedesktop/secrets/collection/default_keyring"
	fakePromptPrefix                   = "/org/freedesktop/secrets/prompt/p"
	ssTestService                      = "com.example.unison.secretservice"
	ssTestAccount                      = "alice"
	ssOtherAccount                     = "bob"
	ssThirdAccount                     = "carol"
)

// fakeItem is one item the fake service holds.
type fakeItem struct {
	attrs map[string]string
	label string
	// collection is the path of the collection that holds the item; addItem puts an item without one in fakeLogin.
	collection dbus.ObjectPath
	value      []byte
	locked     bool
	// stray makes every search return the item whatever its attributes, as a service that matches loosely would.
	stray bool
}

// fakeOwnerChange is a NameOwnerChanged signal the fake sends while a prompt is showing.
type fakeOwnerChange struct {
	sender   string
	name     string
	newOwner string
}

// fakeSecretService is the far end of a connection: a Secret Service with an in-memory item table, which does the
// server side of the key exchange with the same helpers the store uses, answers the bus's match rule calls itself and
// records what it was asked. Every field after the first three is guarded by mu, which the goroutine serving the
// connection holds while it handles a call.
type fakeSecretService struct {
	t        *testing.T
	side     net.Conn
	client   *dbus.Conn
	items    map[dbus.ObjectPath]*fakeItem
	sessions map[dbus.ObjectPath][]byte // A nil key is a plain session
	prompts  map[dbus.ObjectPath]func() (dismissed bool, result dbus.Variant)
	// refuse maps a member to the error name every call of it is answered with; "*" covers every member.
	refuse map[string]string
	// delays maps a member to how long the service takes to answer a call of it.
	delays map[string]time.Duration
	// after maps a member to what runs once the service has answered a call of it, with the lock held.
	after map[string]func()
	// afterDelete runs after an item has been deleted, with the lock held.
	afterDelete func()
	// owner is the sender stamped on everything the service sends.
	owner string
	// forger, when set, sends a Completed signal that dismisses the prompt ahead of the service's own.
	forger string
	// hangUpOn is the member whose call makes the service close the connection rather than answer.
	hangUpOn string
	// alias is the collection the default alias names, or empty when there is none.
	alias dbus.ObjectPath
	// noAliasError is the error name for a call on the default alias when there is none.
	noAliasError string
	// unlockNoAliasError is the error name Unlock answers when asked for the default alias and there is none, or empty
	// for Unlock to skip it, as GNOME Keyring and KeePassXC skip an object they do not know.
	unlockNoAliasError string
	// goneError is the error name for a call on an item that does not exist.
	goneError string
	// log holds the member of every call, with the rule after AddMatch and RemoveMatch, in the order they arrived.
	log         []string
	calls       []*dbus.Message
	plainInputs []dbus.Variant
	// ownerChanges are sent after the reply to a Prompt call, ahead of the prompt's Completed signal.
	ownerChanges []fakeOwnerChange
	itemCount    int
	promptCount  int
	sessionCount int
	serial       uint32
	mu           sync.Mutex
	// collectionLocked makes the default collection locked until a prompt unlocks it.
	collectionLocked bool
	refuseDH         bool
	dismiss          bool
	hangUpOnPrompt   bool
	deletePrompts    bool
	// collectionPrompts makes CreateCollection hand out a prompt, as a service that asks for the new keyring's password
	// does.
	collectionPrompts bool
	// abandonPrompts makes the service never complete a prompt, as one that has quit cannot.
	abandonPrompts bool
}

func newFakeSecretService(t *testing.T) *fakeSecretService {
	t.Helper()
	clientSide, peerSide := net.Pipe()
	client, err := dbus.Connect(clientSide)
	check.New(t).NoError(err)
	f := &fakeSecretService{
		t:            t,
		side:         peerSide,
		client:       client,
		items:        make(map[dbus.ObjectPath]*fakeItem),
		sessions:     make(map[dbus.ObjectPath][]byte),
		prompts:      make(map[dbus.ObjectPath]func() (bool, dbus.Variant)),
		refuse:       make(map[string]string),
		delays:       make(map[string]time.Duration),
		after:        make(map[string]func()),
		owner:        fakeServiceOwner,
		alias:        fakeLogin,
		noAliasError: ssNoSuchObjectError,
		goneError:    ssNoSuchObjectError,
	}
	t.Cleanup(func() {
		client.Close()
		xio.CloseIgnoringErrors(peerSide)
	})
	go f.run()
	return f
}

// store returns a store that reaches the fake through its connection.
func (f *fakeSecretService) store() *secretServiceStore {
	return newSecretServiceStore(func() (*dbus.Conn, error) { return f.client, nil })
}

// configure runs change with the fake's lock held.
func (f *fakeSecretService) configure(change func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change()
}

// addItem puts an item in the fake's table and returns its path.
func (f *fakeSecretService) addItem(item *fakeItem) dbus.ObjectPath {
	f.mu.Lock()
	defer f.mu.Unlock()
	if item.collection == "" {
		item.collection = fakeLogin
	}
	path := f.newItemPath(item.collection)
	f.items[path] = item
	return path
}

// newItemPath returns the path for another item in the collection.
func (f *fakeSecretService) newItemPath(collection dbus.ObjectPath) dbus.ObjectPath {
	f.itemCount++
	return dbus.ObjectPath(string(collection) + "/" + strconv.Itoa(f.itemCount))
}

// item returns a copy of the item at path, or nil.
func (f *fakeSecretService) item(path dbus.ObjectPath) *fakeItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	item := f.items[path]
	if item == nil {
		return nil
	}
	dup := *item
	dup.value = bytes.Clone(item.value)
	return &dup
}

// itemPaths returns the paths of the fake's items, sorted.
func (f *fakeSecretService) itemPaths() []dbus.ObjectPath {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Sorted(maps.Keys(f.items))
}

// forgetSessions drops every session, as a service that has restarted has.
func (f *fakeSecretService) forgetSessions() {
	f.mu.Lock()
	defer f.mu.Unlock()
	clear(f.sessions)
}

// history returns what the fake has been asked so far, and forgets it.
func (f *fakeSecretService) history() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	log := f.log
	f.log = nil
	return log
}

// lastCall returns the most recent call of the member.
func (f *fakeSecretService) lastCall(member string) *dbus.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.calls) - 1; i >= 0; i-- {
		if f.calls[i].Member == member {
			return f.calls[i]
		}
	}
	f.t.Errorf("%s was never called", member)
	return nil
}

func (f *fakeSecretService) run() {
	in := bufio.NewReader(f.side)
	for {
		msg, err := dbus.Decode(in)
		if err != nil {
			return
		}
		if msg.Type != dbus.TypeMethodCall {
			continue
		}
		f.mu.Lock()
		delay := f.delays[msg.Member]
		if delay == 0 {
			f.handle(msg)
		}
		f.mu.Unlock()
		if delay != 0 {
			time.AfterFunc(delay, func() {
				f.mu.Lock()
				defer f.mu.Unlock()
				f.handle(msg)
			})
		}
	}
}

func (f *fakeSecretService) handle(msg *dbus.Message) {
	if msg.Interface == fakeBusName && (msg.Member == "AddMatch" || msg.Member == "RemoveMatch") {
		args, ok := f.args(msg, "s")
		if !ok {
			return
		}
		f.log = append(f.log, msg.Member+" "+fakeArg[string](f, args, 0))
		var reply *dbus.Message
		if name, refused := f.refuse[msg.Member]; refused {
			reply = dbus.NewError(msg, name, "refused by the fake bus")
		} else {
			reply = dbus.NewReply(msg)
		}
		reply.Sender = fakeBusName
		f.write(reply)
		return
	}
	f.log = append(f.log, msg.Member)
	f.calls = append(f.calls, msg)
	if msg.Member == f.hangUpOn {
		xio.CloseIgnoringErrors(f.side)
		return
	}
	if name, refused := f.refuse[msg.Member]; refused {
		f.fail(msg, name)
		return
	}
	if name, refused := f.refuse["*"]; refused {
		f.fail(msg, name)
		return
	}
	switch msg.Interface + "." + msg.Member {
	case ssServiceIface + "." + ssOpenSession:
		f.openSession(msg)
	case ssServiceIface + "." + ssSearchItems:
		f.searchItems(msg)
	case ssServiceIface + "." + ssUnlock:
		f.unlock(msg)
	case ssCollectionIface + "." + ssCreateItem:
		f.createItem(msg)
	case ssItemIface + "." + ssGetSecret:
		f.getSecret(msg)
	case ssItemIface + "." + ssDelete:
		f.deleteItem(msg)
	case dbusPropertiesIface + "." + dbusPropertiesGet:
		f.getProperty(msg)
	case ssServiceIface + "." + ssReadAlias:
		f.readAlias(msg)
	case ssServiceIface + "." + ssCreateCollection:
		f.createCollection(msg)
	case ssPromptIface + "." + ssPrompt:
		f.prompt(msg)
	default:
		f.fail(msg, dbus.UnknownMethod)
	}
	if hook := f.after[msg.Member]; hook != nil {
		hook()
	}
}

func (f *fakeSecretService) openSession(msg *dbus.Message) {
	args, ok := f.args(msg, "sv")
	if !ok {
		return
	}
	f.sessionCount++
	path := dbus.ObjectPath("/org/freedesktop/secrets/session/s" + strconv.Itoa(f.sessionCount))
	input := fakeArg[dbus.Variant](f, args, 1)
	switch fakeArg[string](f, args, 0) {
	case plainAlgorithm:
		f.plainInputs = append(f.plainInputs, input)
		f.sessions[path] = nil
		f.reply(msg, "vo", dbus.Variant{Value: "", Sig: "s"}, path)
	case dhAlgorithm:
		if f.refuseDH {
			f.fail(msg, dbus.NotSupported)
			return
		}
		clientPub, isBytes := input.Value.([]byte)
		if !isBytes {
			f.t.Errorf("the key exchange sent a %q value rather than bytes", input.Sig)
			f.fail(msg, dbus.InvalidArgs)
			return
		}
		priv, pub, err := newDHKeyPair(rand.Reader)
		if err != nil {
			f.t.Errorf("the fake could not make a key pair: %v", err)
			return
		}
		shared, err := dhSharedSecret(priv, new(big.Int).SetBytes(clientPub))
		if err != nil {
			f.t.Errorf("the store sent an unusable public value: %v", err)
			f.fail(msg, dbus.InvalidArgs)
			return
		}
		if f.sessions[path], err = deriveAESKey(shared); err != nil {
			f.t.Errorf("the fake could not derive the key: %v", err)
			return
		}
		f.reply(msg, "vo", dbus.Variant{Value: pub.Bytes(), Sig: "ay"}, path)
	default:
		f.fail(msg, dbus.NotSupported)
	}
}

func (f *fakeSecretService) searchItems(msg *dbus.Message) {
	args, ok := f.args(msg, "a{ss}")
	if !ok {
		return
	}
	query := fakeStrings(f, fakeArg[dbus.Dict](f, args, 0))
	unlocked := make([]dbus.ObjectPath, 0)
	locked := make([]dbus.ObjectPath, 0)
	for _, path := range slices.Sorted(maps.Keys(f.items)) {
		item := f.items[path]
		if !item.stray && !fakeMatches(item.attrs, query) {
			continue
		}
		if item.locked {
			locked = append(locked, path)
		} else {
			unlocked = append(unlocked, path)
		}
	}
	f.reply(msg, "aoao", unlocked, locked)
}

// fakeMatches reports whether attrs holds every attribute in query.
func fakeMatches(attrs, query map[string]string) bool {
	for name, value := range query {
		if attrs[name] != value {
			return false
		}
	}
	return true
}

func (f *fakeSecretService) unlock(msg *dbus.Message) {
	args, ok := f.args(msg, "ao")
	if !ok {
		return
	}
	objects := fakeArg[[]dbus.ObjectPath](f, args, 0)
	already := make([]dbus.ObjectPath, 0, len(objects))
	pending := make([]dbus.ObjectPath, 0, len(objects))
	for _, path := range objects {
		item := f.items[path]
		switch {
		case path == ssDefaultCollection && f.alias == "":
			if f.unlockNoAliasError != "" {
				f.fail(msg, f.unlockNoAliasError)
				return
			}
		case (path == ssDefaultCollection && f.collectionLocked) || (item != nil && item.locked):
			pending = append(pending, path)
		default:
			already = append(already, path)
		}
	}
	if len(pending) == 0 {
		f.reply(msg, "aoo", already, noPrompt)
		return
	}
	prompt := f.newPrompt(func() (bool, dbus.Variant) {
		if f.dismiss {
			return true, dbus.Variant{Value: []dbus.ObjectPath{}, Sig: "ao"}
		}
		for _, path := range pending {
			if path == ssDefaultCollection {
				f.collectionLocked = false
			} else if item := f.items[path]; item != nil {
				item.locked = false
			}
		}
		return false, dbus.Variant{Value: pending, Sig: "ao"}
	})
	f.reply(msg, "aoo", already, prompt)
}

func (f *fakeSecretService) createItem(msg *dbus.Message) {
	args, ok := f.args(msg, "a{sv}(oayays)b")
	if !ok {
		return
	}
	if msg.Path != ssDefaultCollection {
		f.fail(msg, ssNoSuchObjectError)
		return
	}
	if f.alias == "" {
		f.fail(msg, f.noAliasError)
		return
	}
	if f.collectionLocked {
		f.fail(msg, ssIsLockedError)
		return
	}
	var label string
	var attrs map[string]string
	for _, entry := range fakeArg[dbus.Dict](f, args, 0) {
		name, nameOK := entry.Key.(string)
		value, valueOK := entry.Value.(dbus.Variant)
		if !nameOK || !valueOK {
			f.t.Errorf("CreateItem was sent a malformed property %v", entry)
			continue
		}
		switch name {
		case ssLabelProp:
			if text, isText := value.Value.(string); isText {
				label = text
			}
		case ssAttributesProp:
			if dict, isDict := value.Value.(dbus.Dict); isDict {
				attrs = fakeStrings(f, dict)
			}
		default:
		}
	}
	value, known := f.reveal(msg, fakeArg[dbus.Struct](f, args, 1))
	if !known {
		return
	}
	if fakeArg[bool](f, args, 2) {
		for _, path := range slices.Sorted(maps.Keys(f.items)) {
			if item := f.items[path]; item.collection == f.alias && maps.Equal(item.attrs, attrs) {
				item.label = label
				item.value = value
				f.reply(msg, "oo", path, noPrompt)
				return
			}
		}
	}
	path := f.newItemPath(f.alias)
	f.items[path] = &fakeItem{attrs: attrs, label: label, collection: f.alias, value: value}
	f.reply(msg, "oo", path, noPrompt)
}

func (f *fakeSecretService) readAlias(msg *dbus.Message) {
	args, ok := f.args(msg, "s")
	if !ok {
		return
	}
	if fakeArg[string](f, args, 0) != ssDefaultAlias || f.alias == "" {
		f.reply(msg, "o", noCollection)
		return
	}
	f.reply(msg, "o", f.alias)
}

// createCollection creates fakeDefaultKeyring and makes the default alias name it, at once or once its prompt has been
// answered.
func (f *fakeSecretService) createCollection(msg *dbus.Message) {
	if _, ok := f.args(msg, "a{sv}s"); !ok {
		return
	}
	if !f.collectionPrompts {
		f.alias = fakeDefaultKeyring
		f.reply(msg, "oo", f.alias, noPrompt)
		return
	}
	f.reply(msg, "oo", noCollection, f.newPrompt(func() (bool, dbus.Variant) {
		if f.dismiss {
			return true, dbus.Variant{Value: noCollection, Sig: "o"}
		}
		f.alias = fakeDefaultKeyring
		return false, dbus.Variant{Value: f.alias, Sig: "o"}
	}))
}

// reveal returns the value a Secret structure carries, answering the call and returning false when the session is
// not one the service knows.
func (f *fakeSecretService) reveal(msg *dbus.Message, sealed dbus.Struct) ([]byte, bool) {
	session := fakeArg[dbus.ObjectPath](f, sealed, 0)
	iv := fakeArg[[]byte](f, sealed, 1)
	value := fakeArg[[]byte](f, sealed, 2)
	key, known := f.sessions[session]
	if !known {
		f.fail(msg, ssNoSessionError)
		return nil, false
	}
	if key == nil {
		return bytes.Clone(value), true
	}
	plaintext, err := decryptCBC(key, iv, value)
	if err != nil {
		f.t.Errorf("the fake could not decrypt what the store sent: %v", err)
		f.fail(msg, dbus.InvalidArgs)
		return nil, false
	}
	return plaintext, true
}

func (f *fakeSecretService) getSecret(msg *dbus.Message) {
	args, ok := f.args(msg, "o")
	if !ok {
		return
	}
	item := f.items[msg.Path]
	if item == nil {
		f.fail(msg, f.goneError)
		return
	}
	if item.locked {
		f.fail(msg, ssIsLockedError)
		return
	}
	session := fakeArg[dbus.ObjectPath](f, args, 0)
	key, known := f.sessions[session]
	if !known {
		f.fail(msg, ssNoSessionError)
		return
	}
	if key == nil {
		f.reply(msg, "(oayays)", dbus.Struct{session, []byte{}, bytes.Clone(item.value), "text/plain"})
		return
	}
	iv, ciphertext, err := encryptCBC(key, item.value)
	if err != nil {
		f.t.Errorf("the fake could not encrypt: %v", err)
		return
	}
	f.reply(msg, "(oayays)", dbus.Struct{session, iv, ciphertext, "text/plain"})
}

func (f *fakeSecretService) deleteItem(msg *dbus.Message) {
	item := f.items[msg.Path]
	if item == nil {
		f.fail(msg, f.goneError)
		return
	}
	if item.locked {
		f.fail(msg, ssIsLockedError)
		return
	}
	if !f.deletePrompts {
		f.remove(msg.Path)
		f.reply(msg, "o", noPrompt)
		return
	}
	path := msg.Path
	f.reply(msg, "o", f.newPrompt(func() (bool, dbus.Variant) {
		if f.dismiss {
			return true, dbus.Variant{Value: "", Sig: "s"}
		}
		f.remove(path)
		return false, dbus.Variant{Value: "", Sig: "s"}
	}))
}

// remove deletes the item at path and then runs afterDelete, if set.
func (f *fakeSecretService) remove(path dbus.ObjectPath) {
	delete(f.items, path)
	if f.afterDelete != nil {
		f.afterDelete()
	}
}

func (f *fakeSecretService) getProperty(msg *dbus.Message) {
	args, ok := f.args(msg, "ss")
	if !ok {
		return
	}
	item := f.items[msg.Path]
	if item == nil {
		f.fail(msg, f.goneError)
		return
	}
	if fakeArg[string](f, args, 0) != ssItemIface || fakeArg[string](f, args, 1) != "Attributes" {
		f.fail(msg, dbus.UnknownProperty)
		return
	}
	f.reply(msg, "v", dbus.Variant{Value: item.attrs, Sig: "a{ss}"})
}

func (f *fakeSecretService) prompt(msg *dbus.Message) {
	args, ok := f.args(msg, "s")
	if !ok {
		return
	}
	if windowID := fakeArg[string](f, args, 0); windowID != "" {
		f.t.Errorf("Prompt was passed the window %q rather than none", windowID)
	}
	action, exists := f.prompts[msg.Path]
	if !exists {
		f.fail(msg, ssNoSuchObjectError)
		return
	}
	delete(f.prompts, msg.Path)
	f.reply(msg, "")
	if f.hangUpOnPrompt {
		xio.CloseIgnoringErrors(f.side)
		return
	}
	if f.forger != "" {
		f.emitCompleted(f.forger, msg.Path, true, dbus.Variant{Value: []dbus.ObjectPath{}, Sig: "ao"})
	}
	for _, change := range f.ownerChanges {
		signal := dbus.NewSignal(fakeBusPath, fakeBusName, "NameOwnerChanged")
		signal.Sender = change.sender
		if err := signal.SetBodyWithSignature("sss", change.name, f.owner, change.newOwner); err != nil {
			f.t.Errorf("the fake could not marshal a NameOwnerChanged signal: %v", err)
			return
		}
		f.write(signal)
	}
	if f.abandonPrompts {
		return
	}
	dismissed, result := action()
	f.emitCompleted(f.owner, msg.Path, dismissed, result)
}

// newPrompt registers what happens when the prompt it returns the path of is shown.
func (f *fakeSecretService) newPrompt(action func() (bool, dbus.Variant)) dbus.ObjectPath {
	f.promptCount++
	path := dbus.ObjectPath(fakePromptPrefix + strconv.Itoa(f.promptCount))
	f.prompts[path] = action
	return path
}

func (f *fakeSecretService) emitCompleted(sender string, prompt dbus.ObjectPath, dismissed bool, result dbus.Variant) {
	msg := dbus.NewSignal(prompt, ssPromptIface, ssCompleted)
	msg.Sender = sender
	if err := msg.SetBodyWithSignature("bv", dismissed, result); err != nil {
		f.t.Errorf("the fake could not marshal a Completed signal: %v", err)
		return
	}
	f.write(msg)
}

// args returns the arguments of a call, answering it with InvalidArgs and failing the test when their signature is not
// the one the Secret Service API gives the method.
func (f *fakeSecretService) args(msg *dbus.Message, sig dbus.Signature) ([]any, bool) {
	args, err := replyArgs(msg, sig)
	if err != nil {
		f.t.Errorf("%s: %v", msg, err)
		f.fail(msg, dbus.InvalidArgs)
		return nil, false
	}
	return args, true
}

func (f *fakeSecretService) reply(call *dbus.Message, sig dbus.Signature, args ...any) {
	reply := dbus.NewReply(call)
	reply.Sender = f.owner
	if sig != "" {
		if err := reply.SetBodyWithSignature(sig, args...); err != nil {
			f.t.Errorf("the fake could not marshal its reply to %s: %v", call, err)
			return
		}
	}
	f.write(reply)
}

func (f *fakeSecretService) fail(call *dbus.Message, name string) {
	reply := dbus.NewError(call, name, "refused by the fake Secret Service")
	reply.Sender = f.owner
	f.write(reply)
}

func (f *fakeSecretService) write(msg *dbus.Message) {
	f.serial++
	msg.Serial = f.serial
	data, err := msg.Encode()
	if err != nil {
		f.t.Errorf("the fake could not encode %s: %v", msg, err)
		return
	}
	if _, err = f.side.Write(data); err != nil {
		return // The connection under test has gone, which the end of every test brings about on purpose
	}
}

// fakeArg returns args[i] as a T, failing the test when it is something else.
func fakeArg[T any](f *fakeSecretService, args []any, i int) T {
	v, err := replyArg[T](args, i)
	if err != nil {
		f.t.Errorf("the fake was sent a malformed call: %v", err)
	}
	return v
}

// fakeStrings returns an a{ss} dictionary as a map.
func fakeStrings(f *fakeSecretService, dict dbus.Dict) map[string]string {
	m := make(map[string]string, len(dict))
	for _, entry := range dict {
		name, nameOK := entry.Key.(string)
		value, valueOK := entry.Value.(string)
		if !nameOK || !valueOK {
			f.t.Errorf("the fake was sent a malformed attribute %v", entry)
			continue
		}
		m[name] = value
	}
	return m
}

// ssPromptHistory returns the history entries for showing the prompt: the match rules it needs being added, the call,
// and the rules being removed.
func ssPromptHistory(prompt dbus.ObjectPath) []string {
	completed := ssPromptRule(prompt)
	return []string{
		"AddMatch " + completed,
		"AddMatch " + ssOwnerRule,
		ssPrompt,
		"RemoveMatch " + ssOwnerRule,
		"RemoveMatch " + completed,
	}
}

// ssOwnerRule is the match rule the store must ask for to learn that the service has gone while a prompt is showing.
const ssOwnerRule = "type='signal',sender='org.freedesktop.DBus',interface='org.freedesktop.DBus'," +
	"member='NameOwnerChanged',arg0='org.freedesktop.secrets'"

// ssPromptRule returns the match rule the store must ask for while the prompt is showing.
func ssPromptRule(prompt dbus.ObjectPath) string {
	return "type='signal',interface='org.freedesktop.Secret.Prompt',member='Completed',path='" + string(prompt) + "'"
}

// ssDo runs op on a goroutine of its own and returns its error, failing the test if it has not finished within
// ssTestTimeout: a prompt waits for as long as the user takes, so a bug would otherwise hang the suite.
func ssDo(t *testing.T, op func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- op() }()
	select {
	case err := <-done:
		return err
	case <-time.After(ssTestTimeout):
		t.Fatal("timed out waiting for a Secret Service operation")
		return nil
	}
}

func ssDoSet(t *testing.T, store *secretServiceStore, account string, value []byte) error {
	t.Helper()
	return ssDo(t, func() error { return store.Set(ssTestService, account, value) })
}

func ssDoGet(t *testing.T, store *secretServiceStore, account string) ([]byte, error) {
	t.Helper()
	var value []byte
	err := ssDo(t, func() error {
		var getErr error
		value, getErr = store.Get(ssTestService, account)
		return getErr
	})
	return value, err
}

func ssDoDelete(t *testing.T, store *secretServiceStore, account string) error {
	t.Helper()
	return ssDo(t, func() error { return store.Delete(ssTestService, account) })
}

func ssDoAccounts(t *testing.T, store *secretServiceStore, service string) ([]string, error) {
	t.Helper()
	var accounts []string
	err := ssDo(t, func() error {
		var accountsErr error
		accounts, accountsErr = store.Accounts(service)
		return accountsErr
	})
	return accounts, err
}

// ssConnFieldLen returns the length of one of the connection's unexported slices. A subscription left behind, or a
// second OnDisconnect handler, does nothing that can be observed from outside, so the tests look inside.
func ssConnFieldLen(t *testing.T, conn *dbus.Conn, name string) int {
	t.Helper()
	field := reflect.ValueOf(conn).Elem().FieldByName(name)
	if field.Kind() != reflect.Slice {
		t.Fatalf("dbus.Conn no longer keeps a slice named %s", name)
	}
	return field.Len()
}

// ssCount returns how many times member appears in a history.
func ssCount(history []string, member string) int {
	n := 0
	for _, one := range history {
		if one == member {
			n++
		}
	}
	return n
}

func TestSecretServiceEncryptedSession(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	store := f.store()
	payload := []byte("correct horse battery staple")

	c.NoError(ssDoSet(t, store, ssTestAccount, payload))
	c.Equal([]string{ssOpenSession, ssUnlock, ssCreateItem}, f.history())
	paths := f.itemPaths()
	c.Equal(1, len(paths))
	c.Equal(payload, f.item(paths[0]).value, "the service must decrypt what the store sent")
	created := f.lastCall(ssCreateItem)
	args, err := created.Args()
	c.NoError(err)
	sealed, isStruct := args[1].(dbus.Struct)
	c.True(isStruct)
	c.Equal(4, len(sealed))
	iv, ivOK := sealed[1].([]byte)
	ciphertext, ciphertextOK := sealed[2].([]byte)
	c.True(ivOK && ciphertextOK)
	c.Equal(aes.BlockSize, len(iv))
	c.Equal(0, len(ciphertext)%aes.BlockSize)
	c.False(bytes.Contains(ciphertext, payload), "the secret must not cross the bus in the clear")
	c.Equal(ssContentType, sealed[3])
	opened := f.lastCall(ssOpenSession)
	args, err = opened.Args()
	c.NoError(err)
	c.Equal(dhAlgorithm, args[0])

	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal(payload, value, "the store must decrypt what the service sent")
	c.Equal([]string{ssSearchItems, dbusPropertiesGet, ssGetSecret}, f.history())

	c.NoError(ssDoSet(t, store, ssTestAccount, nil))
	value, err = ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.NotNil(value)
	c.Equal(0, len(value))
	c.Equal(1, len(f.itemPaths()), "replacing a secret must not add an item")
}

func TestSecretServicePlainSessionFallback(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.configure(func() { f.refuseDH = true })
	store := f.store()
	payload := []byte("in the clear")

	c.NoError(ssDoSet(t, store, ssTestAccount, payload))
	c.Equal([]string{ssOpenSession, ssOpenSession, ssUnlock, ssCreateItem}, f.history())
	f.configure(func() {
		c.Equal([]dbus.Variant{{Value: "", Sig: "s"}}, f.plainInputs)
	})
	args, err := f.lastCall(ssOpenSession).Args()
	c.NoError(err)
	c.Equal(plainAlgorithm, args[0])
	args, err = f.lastCall(ssCreateItem).Args()
	c.NoError(err)
	sealed, isStruct := args[1].(dbus.Struct)
	c.True(isStruct)
	c.Equal([]byte{}, sealed[1], "a plain session sends no IV")
	c.Equal(payload, sealed[2])

	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal(payload, value)
}

func TestSecretServiceSetCreatesItem(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	store := f.store()
	c.NoError(ssDoSet(t, store, "bob:work", []byte("one")))

	created := f.lastCall(ssCreateItem)
	c.Equal(ssDefaultCollection, created.Path)
	c.Equal(ssCollectionIface, created.Interface)
	c.Equal(dbus.Signature("a{sv}(oayays)b"), created.Signature)
	args, err := created.Args()
	c.NoError(err)
	c.Equal(3, len(args))
	c.Equal(true, args[2], "an existing secret must be replaced")
	props, isDict := args[0].(dbus.Dict)
	c.True(isDict)
	c.Equal(dbus.Dict{
		{Key: ssLabelProp, Value: dbus.Variant{Value: ssTestService + ": bob:work", Sig: "s"}},
		{Key: ssAttributesProp, Value: dbus.Variant{Value: dbus.Dict{
			{Key: ssAccountAttr, Value: "bob:work"},
			{Key: ssServiceAttr, Value: ssTestService},
		}, Sig: "a{ss}"}},
	}, props)
	unlocked := f.lastCall(ssUnlock)
	args, err = unlocked.Args()
	c.NoError(err)
	c.Equal([]dbus.ObjectPath{ssDefaultCollection}, args[0])

	c.NoError(ssDoSet(t, store, "bob:work", []byte("two")))
	paths := f.itemPaths()
	c.Equal(1, len(paths))
	item := f.item(paths[0])
	c.Equal([]byte("two"), item.value)
	c.Equal(ssTestService+": bob:work", item.label)
	c.Equal(map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: "bob:work"}, item.attrs)
}

func TestSecretServiceSetUnlocksTheCollection(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.configure(func() { f.collectionLocked = true })
	store := f.store()

	c.NoError(ssDoSet(t, store, ssTestAccount, []byte("unlocked")))
	prompt := dbus.ObjectPath(fakePromptPrefix + "1")
	c.Equal(slices.Concat([]string{ssOpenSession, ssUnlock}, ssPromptHistory(prompt), []string{ssCreateItem}),
		f.history())
	c.Equal(prompt, f.lastCall(ssPrompt).Path)
	c.Equal(0, ssConnFieldLen(t, f.client, "subs"), "the prompt's subscriptions must be canceled")
	paths := f.itemPaths()
	c.Equal(1, len(paths))
	c.Equal([]byte("unlocked"), f.item(paths[0]).value)
}

// TestSecretServiceSetCreatesTheDefaultCollection covers a service with no default collection, as GNOME Keyring started
// without PAM has: Set must create one, as libsecret does, whether the service answers the missing alias with
// NoSuchObject or, as GDBus and QtDBus do for a path that has nothing at it, UnknownMethod or UnknownObject, and
// whether Unlock skips the missing alias or refuses it.
func TestSecretServiceSetCreatesTheDefaultCollection(t *testing.T) {
	t.Parallel()
	for _, noAliasError := range []string{ssNoSuchObjectError, dbus.UnknownObject, dbus.UnknownMethod} {
		for _, unlockNoAliasError := range []string{"", ssNoSuchObjectError, dbus.UnknownObject} {
			for _, prompts := range []bool{false, true} {
				c := check.New(t)
				f := newFakeSecretService(t)
				f.configure(func() {
					f.alias = ""
					f.noAliasError = noAliasError
					f.unlockNoAliasError = unlockNoAliasError
					f.collectionPrompts = prompts
				})
				store := f.store()
				what := noAliasError + " " + unlockNoAliasError
				c.NoError(ssDoSet(t, store, ssTestAccount, []byte("first")), what)
				expected := []string{ssOpenSession, ssUnlock, ssCreateItem, ssCreateCollection}
				if prompts {
					expected = append(expected, ssPromptHistory(fakePromptPrefix+"1")...)
				}
				c.Equal(append(expected, ssCreateItem), f.history(), what)
				if created := f.lastCall(ssCreateCollection); created != nil {
					args, err := created.Args()
					c.NoError(err)
					c.Equal([]any{
						dbus.Dict{{Key: ssCollectionLabel, Value: dbus.Variant{Value: "Default keyring", Sig: "s"}}},
						ssDefaultAlias,
					}, args)
				}
				c.Equal(ssDefaultCollection, f.lastCall(ssCreateItem).Path)
				paths := f.itemPaths()
				c.Equal(1, len(paths))
				c.Equal(fakeDefaultKeyring, f.item(paths[0]).collection)
				value, err := ssDoGet(t, store, ssTestAccount)
				c.NoError(err)
				c.Equal([]byte("first"), value)
			}
		}
	}

	c := check.New(t)
	f := newFakeSecretService(t)
	f.configure(func() {
		f.alias = ""
		f.collectionPrompts = true
		f.dismiss = true
	})
	err := ssDoSet(t, f.store(), ssTestAccount, []byte("never"))
	c.True(errors.Is(err, ErrPromptDismissed))
	c.Equal(1, ssCount(f.history(), ssCreateItem), "CreateItem must not be retried once creating the collection fails")
	c.Equal(0, len(f.itemPaths()))
}

func TestSecretServiceGet(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount},
		value: []byte("bob's"),
	})
	f.addItem(&fakeItem{
		attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value: []byte("alice's"),
	})
	store := f.store()

	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("alice's"), value)
	c.Equal([]string{ssOpenSession, ssSearchItems, dbusPropertiesGet, ssGetSecret}, f.history())
	args, err := f.lastCall(ssSearchItems).Args()
	c.NoError(err)
	c.Equal(dbus.Dict{{Key: ssAccountAttr, Value: ssTestAccount}, {Key: ssServiceAttr, Value: ssTestService}}, args[0])

	_, err = ssDoGet(t, store, ssThirdAccount)
	c.True(errors.Is(err, ErrNotFound))
	c.Equal([]string{ssSearchItems}, f.history())
}

// TestSecretServiceGetPrefersTheDefaultCollection covers a matching item in more than one collection, as after the user
// makes another collection the default between two calls of Set: Get must read the item in the collection the default
// alias names, since that is the one Set replaces, even when that collection is locked and the other is not.
func TestSecretServiceGetPrefersTheDefaultCollection(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	store := f.store()
	c.NoError(ssDoSet(t, store, ssTestAccount, []byte("old")))
	f.configure(func() { f.alias = fakeWork })
	c.NoError(ssDoSet(t, store, ssTestAccount, []byte("new")))
	c.Equal(2, len(f.itemPaths()))
	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("new"), value)
	if read := f.lastCall(ssReadAlias); read != nil {
		args, argsErr := read.Args()
		c.NoError(argsErr)
		c.Equal([]any{ssDefaultAlias}, args)
	}

	f.configure(func() {
		for _, item := range f.items {
			item.locked = item.collection == fakeWork
		}
	})
	f.history()
	value, err = ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("new"), value)
	c.Equal(1, ssCount(f.history(), ssUnlock))

	// With no default collection, the first match is read.
	f.configure(func() { f.alias = "" })
	value, err = ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("old"), value)
}

func TestSecretServiceGetUnlocksTheItem(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	path := f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value:  []byte("locked away"),
		locked: true,
	})
	store := f.store()

	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("locked away"), value)
	prompt := dbus.ObjectPath(fakePromptPrefix + "1")
	c.Equal(slices.Concat([]string{ssOpenSession, ssSearchItems, ssUnlock}, ssPromptHistory(prompt),
		[]string{dbusPropertiesGet, ssGetSecret}), f.history())
	args, err := f.lastCall(ssUnlock).Args()
	c.NoError(err)
	c.Equal([]dbus.ObjectPath{path}, args[0])
	c.False(f.item(path).locked)
}

func TestSecretServiceDelete(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	attrs := map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}
	f.addItem(&fakeItem{attrs: attrs, value: []byte("one")})
	f.addItem(&fakeItem{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount}})
	store := f.store()

	c.NoError(ssDoDelete(t, store, ssTestAccount))
	c.Equal([]string{ssOpenSession, ssSearchItems, dbusPropertiesGet, ssDelete}, f.history())
	c.Equal(1, len(f.itemPaths()), "only the account's item may be deleted")
	c.True(errors.Is(ssDoDelete(t, store, ssTestAccount), ErrNotFound))
	c.Equal([]string{ssSearchItems}, f.history())

	// A service that confirms deletion with a prompt, for an item that is locked as well.
	f.addItem(&fakeItem{attrs: attrs, value: []byte("two"), locked: true})
	f.configure(func() { f.deletePrompts = true })
	c.NoError(ssDoDelete(t, store, ssTestAccount))
	unlockPrompt := dbus.ObjectPath(fakePromptPrefix + "1")
	deletePrompt := dbus.ObjectPath(fakePromptPrefix + "2")
	c.Equal(slices.Concat([]string{ssSearchItems, ssUnlock}, ssPromptHistory(unlockPrompt),
		[]string{dbusPropertiesGet, ssDelete}, ssPromptHistory(deletePrompt)), f.history())
	c.Equal(1, len(f.itemPaths()))
	_, err := ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrNotFound))
}

// TestSecretServiceChecksWhatTheSearchFound covers a service whose search matches loosely: Get and Delete must act only
// on an item whose attributes name the service and account, reading a locked item's only once it is unlocked.
func TestSecretServiceChecksWhatTheSearchFound(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	strays := []dbus.ObjectPath{
		f.addItem(&fakeItem{
			attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount + "e"},
			value: []byte("alicee's"),
			stray: true,
		}),
		f.addItem(&fakeItem{
			attrs: map[string]string{ssServiceAttr: "other", ssAccountAttr: ssTestAccount},
			value: []byte("another application's"),
			stray: true,
		}),
		f.addItem(&fakeItem{
			attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount},
			value:  []byte("bob's"),
			locked: true,
			stray:  true,
		}),
	}
	store := f.store()

	_, err := ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrNotFound), "a stray item must not be read")
	c.Equal(1, ssCount(f.history(), ssUnlock), "the locked item must be unlocked before it is checked")
	c.True(errors.Is(ssDoDelete(t, store, ssTestAccount), ErrNotFound), "a stray item must not be deleted")
	c.Equal(0, ssCount(f.history(), ssDelete))
	c.Equal(strays, f.itemPaths())

	f.addItem(&fakeItem{
		attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value: []byte("alice's"),
	})
	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("alice's"), value)
	c.NoError(ssDoDelete(t, store, ssTestAccount))
	c.Equal(strays, f.itemPaths(), "only the account's item may be deleted")
}

func TestSecretServiceAccounts(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	for _, item := range []*fakeItem{
		{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssThirdAccount}},
		{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: "alice"}, locked: true},
		{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssThirdAccount}, locked: true},
		{attrs: map[string]string{ssServiceAttr: "other", ssAccountAttr: "zed"}, stray: true},
		{attrs: map[string]string{ssServiceAttr: "other", ssAccountAttr: "yuri"}},
		{attrs: map[string]string{ssServiceAttr: ssTestService}},
	} {
		f.addItem(item)
	}
	store := f.store()

	accounts, err := ssDoAccounts(t, store, ssTestService)
	c.NoError(err)
	c.Equal([]string{ssTestAccount, ssThirdAccount}, accounts)
	history := f.history()
	c.Equal(5, ssCount(history, dbusPropertiesGet), "every item the search returned must be read once unlocked")
	c.Equal(1, ssCount(history, ssUnlock), "the locked items must be unlocked before their attributes are read")
	c.Equal(1, ssCount(history, ssPrompt))
	args, err := f.lastCall(ssSearchItems).Args()
	c.NoError(err)
	c.Equal(dbus.Dict{{Key: ssServiceAttr, Value: ssTestService}}, args[0])
	args, err = f.lastCall(ssUnlock).Args()
	c.NoError(err)
	toUnlock, ok := args[0].([]dbus.ObjectPath)
	c.True(ok)
	c.Equal(2, len(toUnlock), "only the locked items are unlocked")
	for _, item := range f.itemPaths() {
		c.False(f.item(item).locked, "%s should have been unlocked", item)
	}

	accounts, err = ssDoAccounts(t, store, "nobody")
	c.NoError(err)
	c.Nil(accounts)
}

// TestSecretServiceAccountsWhenTheUnlockIsDismissed covers dismissing the prompt Accounts shows to unlock the locked
// items: the accounts of the unlocked ones, including one the service unlocked without the prompt, must still be
// listed, while any other failure is still reported.
func TestSecretServiceAccountsWhenTheUnlockIsDismissed(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssThirdAccount}})
	f.addItem(&fakeItem{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: "alice"}, locked: true})
	bobs := f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount},
		locked: true,
	})
	f.configure(func() {
		f.dismiss = true
		// Unlocked between the search and the unlock, as when another process unlocks its collection meanwhile
		f.after[ssSearchItems] = func() { f.items[bobs].locked = false }
	})
	store := f.store()
	accounts, err := ssDoAccounts(t, store, ssTestService)
	c.NoError(err)
	c.Equal([]string{ssOtherAccount, ssThirdAccount}, accounts)
	c.Equal(1, ssCount(f.history(), ssPrompt))
	args, err := f.lastCall(ssUnlock).Args()
	c.NoError(err)
	toUnlock, ok := args[0].([]dbus.ObjectPath)
	c.True(ok)
	c.Equal(2, len(toUnlock), "both locked items must be passed to Unlock")

	f.configure(func() { f.refuse[ssUnlock] = dbus.Failed })
	_, err = ssDoAccounts(t, store, ssTestService)
	var dbusErr *dbus.Error
	c.True(errors.As(err, &dbusErr))
	c.Equal(dbus.Failed, dbusErr.Name)
}

func TestSecretServiceUnavailable(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		dbus.ServiceUnknown,
		dbusNameHasNoOwner,
		"org.freedesktop.DBus.Error.Spawn.ChildExited",
		"org.freedesktop.systemd1.NoSuchUnit",
		"org.freedesktop.DBus.Error.NoReply",
	} {
		c := check.New(t)
		f := newFakeSecretService(t)
		f.configure(func() { f.refuse["*"] = name })
		store := f.store()
		c.True(errors.Is(ssDoSet(t, store, ssTestAccount, []byte("x")), ErrUnavailable), name)
		_, err := ssDoGet(t, store, ssTestAccount)
		c.True(errors.Is(err, ErrUnavailable), name)
		c.True(errors.Is(ssDoDelete(t, store, ssTestAccount), ErrUnavailable), name)
		_, err = ssDoAccounts(t, store, ssTestService)
		c.True(errors.Is(err, ErrUnavailable), name)
	}
}

// TestSecretServiceWithoutASessionBus covers the path the platform store takes, through dbus.Session, when there is no
// session bus to reach, and checks that the same path reaches the service once there is one.
func TestSecretServiceWithoutASessionBus(t *testing.T) {
	c := check.New(t)
	restore := dbus.SetSessionForTest(nil, errors.New("no session bus"))
	defer restore()
	store := newSecretServiceStore(dbus.Session)
	c.True(errors.Is(ssDoSet(t, store, ssTestAccount, []byte("x")), ErrUnavailable))
	_, err := ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrUnavailable))
	c.True(errors.Is(ssDoDelete(t, store, ssTestAccount), ErrUnavailable))
	_, err = ssDoAccounts(t, store, ssTestService)
	c.True(errors.Is(err, ErrUnavailable))

	store = newSecretServiceStore(func() (*dbus.Conn, error) { return nil, nil })
	_, err = ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrUnavailable))

	f := newFakeSecretService(t)
	restoreShared := dbus.SetSessionForTest(f.client, nil)
	defer restoreShared()
	store = newSecretServiceStore(dbus.Session)
	c.NoError(ssDoSet(t, store, ssTestAccount, []byte("shared")))
	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("shared"), value)
}

func TestSecretServiceDismissedPrompt(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value:  []byte("still locked"),
		locked: true,
	})
	f.configure(func() { f.dismiss = true })
	store := f.store()

	_, err := ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrPromptDismissed))
	c.False(errors.Is(err, ErrNotFound))
	c.False(errors.Is(err, ErrUnavailable))
	prompt := dbus.ObjectPath(fakePromptPrefix + "1")
	c.Equal(slices.Concat([]string{ssOpenSession, ssSearchItems, ssUnlock}, ssPromptHistory(prompt)), f.history())
	c.Equal(0, ssConnFieldLen(t, f.client, "subs"), "the prompt's subscriptions must be canceled")

	f.configure(func() { f.dismiss = false })
	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("still locked"), value)

	// A match the service unlocks without the prompt must be read although the prompt for the other is dismissed.
	f = newFakeSecretService(t)
	attrs := map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}
	f.addItem(&fakeItem{attrs: attrs, value: []byte("still locked"), locked: true})
	unlocked := f.addItem(&fakeItem{attrs: maps.Clone(attrs), value: []byte("unlocked meanwhile"), locked: true})
	f.configure(func() {
		f.dismiss = true
		f.after[ssSearchItems] = func() { f.items[unlocked].locked = false }
	})
	value, err = ssDoGet(t, f.store(), ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("unlocked meanwhile"), value)
	c.Equal(1, ssCount(f.history(), ssPrompt))
}

// TestSecretServicePromptGone covers a service whose prompt object has gone by the time Prompt is called, answering
// NoSuchObject or, as GDBus and QtDBus do for a path that has nothing at it, UnknownObject or UnknownMethod. It is the
// prompt that is missing, not the secret, so Delete must not skip the item as already deleted, and neither Delete, Get
// nor Set may report ErrNotFound.
func TestSecretServicePromptGone(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, goneError := range []string{ssNoSuchObjectError, dbus.UnknownObject, dbus.UnknownMethod} {
		f := newFakeSecretService(t)
		f.addItem(&fakeItem{
			attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
			value: []byte("kept"),
		})
		f.addItem(&fakeItem{
			attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount},
			value:  []byte("locked"),
			locked: true,
		})
		f.configure(func() {
			f.deletePrompts = true
			f.collectionLocked = true
			f.refuse[ssPrompt] = goneError
		})
		store := f.store()

		err := ssDoDelete(t, store, ssTestAccount)
		c.NotNil(err, goneError)
		c.False(errors.Is(err, ErrNotFound), "%s: %v", goneError, err)
		c.False(errors.Is(err, ErrUnavailable), "%s: %v", goneError, err)
		c.Equal(2, len(f.itemPaths()), "%s: an item whose prompt went must not count as deleted", goneError)
		c.Equal(0, ssConnFieldLen(t, f.client, "subs"), "the prompt's subscriptions must be canceled")

		_, err = ssDoGet(t, store, ssOtherAccount)
		c.NotNil(err, goneError)
		c.False(errors.Is(err, ErrNotFound), "%s: %v", goneError, err)

		err = ssDoSet(t, store, ssTestAccount, []byte("new"))
		c.NotNil(err, goneError)
		c.False(errors.Is(err, ErrNotFound), "%s: %v", goneError, err)
	}
}

func TestSecretServiceConnectionLostDuringPrompt(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		locked: true,
	})
	f.configure(func() { f.hangUpOnPrompt = true })
	store := f.store()

	_, err := ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrUnavailable))
	c.Equal(0, ssConnFieldLen(t, f.client, "subs"), "the prompt's subscriptions must be canceled")
	_, err = ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrUnavailable), "a lost connection must stay unavailable")
}

// TestSecretServicePromptWatchesTheServiceName covers a service that quits, or is replaced, while its prompt is
// showing, after which nothing would ever complete the prompt. A NameOwnerChanged signal from anyone but the bus, or
// about another name, must change nothing.
func TestSecretServicePromptWatchesTheServiceName(t *testing.T) {
	t.Parallel()
	for _, newOwner := range []string{"", ":1.42"} {
		c := check.New(t)
		f := newFakeSecretService(t)
		f.addItem(&fakeItem{
			attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
			locked: true,
		})
		f.configure(func() {
			f.ownerChanges = []fakeOwnerChange{{sender: fakeBusName, name: ssDestination, newOwner: newOwner}}
			f.abandonPrompts = true
		})
		_, err := ssDoGet(t, f.store(), ssTestAccount)
		c.True(errors.Is(err, ErrUnavailable), newOwner)
		c.False(errors.Is(err, ErrPromptDismissed), newOwner)
		prompt := dbus.ObjectPath(fakePromptPrefix + "1")
		c.Equal(slices.Concat([]string{ssOpenSession, ssSearchItems, ssUnlock}, ssPromptHistory(prompt)), f.history(),
			newOwner)
		c.Equal(0, ssConnFieldLen(t, f.client, "subs"), "the prompt's subscriptions must be canceled")
	}

	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value:  []byte("still there"),
		locked: true,
	})
	f.configure(func() {
		f.ownerChanges = []fakeOwnerChange{
			{sender: fakeOtherSender, name: ssDestination},
			{sender: fakeBusName, name: "org.example.other"},
			{sender: fakeBusName, name: ssDestination, newOwner: fakeServiceOwner},
		}
	})
	value, err := ssDoGet(t, f.store(), ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("still there"), value)
}

// TestSecretServiceLogsOnlyARefusedRuleRemoval covers removing the match rules a prompt asked for: failing because the
// connection has gone is expected, even before the store's disconnect handler has run, and must not be logged, while
// the bus refusing is worth logging. It replaces the default logger, so it must not run in parallel.
func TestSecretServiceLogsOnlyARefusedRuleRemoval(t *testing.T) {
	c := check.New(t)
	var logged bytes.Buffer
	prior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	defer slog.SetDefault(prior)
	const complaint = "unable to remove the match rule"
	attrs := map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}

	f := newFakeSecretService(t)
	f.addItem(&fakeItem{attrs: attrs, locked: true})
	f.configure(func() { f.hangUpOn = ssPrompt })
	store := f.store()
	_, err := ssDoAccounts(t, store, "nobody")
	c.NoError(err)
	store.mu.Lock()
	store.disconnected = make(chan struct{}) // Stands in for the one the handler, yet to run, would close
	store.mu.Unlock()
	_, err = ssDoGet(t, store, ssTestAccount)
	c.True(errors.Is(err, ErrUnavailable))
	c.False(strings.Contains(logged.String(), complaint), logged.String())

	f = newFakeSecretService(t)
	f.addItem(&fakeItem{attrs: attrs, locked: true})
	f.configure(func() { f.refuse["RemoveMatch"] = dbus.Failed })
	_, err = ssDoGet(t, f.store(), ssTestAccount)
	c.NoError(err)
	c.True(strings.Contains(logged.String(), complaint), logged.String())
}

func TestSecretServiceConnectionLostDuringCall(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.configure(func() { f.hangUpOn = ssSearchItems })
	_, err := ssDoGet(t, f.store(), ssTestAccount)
	c.True(errors.Is(err, ErrUnavailable))
}

// TestSecretServiceWaitsForASlowService covers a service that holds back its reply to a method while it starts up or
// prompts the user, for longer than the connection waits for an ordinary call.
func TestSecretServiceWaitsForASlowService(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.client.SetTimeoutForTest(50 * time.Millisecond)
	f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount},
		value:  []byte("locked"),
		locked: true,
	})
	f.configure(func() {
		for _, member := range []string{
			ssOpenSession, ssSearchItems, ssUnlock, ssCreateItem, ssCreateCollection, ssGetSecret, ssDelete, ssPrompt,
		} {
			f.delays[member] = 150 * time.Millisecond
		}
		f.alias = ""
		f.collectionPrompts = true
		f.deletePrompts = true
	})
	store := f.store()
	c.NoError(ssDoSet(t, store, ssTestAccount, []byte("slow")))
	value, err := ssDoGet(t, store, ssOtherAccount)
	c.NoError(err)
	c.Equal([]byte("locked"), value)
	c.NoError(ssDoDelete(t, store, ssTestAccount))
	history := f.history()
	for _, member := range []string{ssCreateCollection, ssUnlock, ssPrompt, ssGetSecret, ssDelete} {
		c.True(slices.Contains(history, member), member)
	}
}

func TestSecretServiceReusesItsSession(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value: []byte("again"),
	})
	store := f.store()
	for range 2 {
		value, err := ssDoGet(t, store, ssTestAccount)
		c.NoError(err)
		c.Equal([]byte("again"), value)
	}
	_, err := ssDoAccounts(t, store, ssTestService)
	c.NoError(err)
	c.Equal(1, ssCount(f.history(), ssOpenSession))
	c.Equal(1, ssConnFieldLen(t, f.client, "onDisconnect"), "one OnDisconnect handler per connection")
}

func TestSecretServiceConcurrentUse(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	store := f.store()
	const workers = 8
	var wg sync.WaitGroup
	failures := make(chan error, workers)
	for i := range workers {
		wg.Go(func() {
			account := "account" + strconv.Itoa(i)
			if err := store.Set(ssTestService, account, []byte(account)); err != nil {
				failures <- err
				return
			}
			value, err := store.Get(ssTestService, account)
			if err != nil {
				failures <- err
				return
			}
			if !bytes.Equal([]byte(account), value) {
				failures <- errors.New("read back " + string(value) + " for " + account)
			}
		})
	}
	c.NoError(ssDo(t, func() error {
		wg.Wait()
		return nil
	}))
	close(failures)
	for err := range failures {
		c.NoError(err)
	}
	c.Equal(1, ssCount(f.history(), ssOpenSession), "concurrent first calls must share one session")
	c.Equal(workers, len(f.itemPaths()))
}

func TestSecretServiceIgnoresAForgedCompletedSignal(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs:  map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value:  []byte("genuine"),
		locked: true,
	})
	f.configure(func() { f.forger = fakeOtherSender })
	value, err := ssDoGet(t, f.store(), ssTestAccount)
	c.NoError(err, "the forged dismissal must be ignored")
	c.Equal([]byte("genuine"), value)
}

func TestSecretServiceReopensAStaleSession(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{
		attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount},
		value: []byte("survives"),
	})
	store := f.store()
	_, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	f.history()

	f.forgetSessions()
	value, err := ssDoGet(t, store, ssTestAccount)
	c.NoError(err)
	c.Equal([]byte("survives"), value)
	c.Equal([]string{
		ssSearchItems,
		dbusPropertiesGet,
		ssGetSecret,
		ssOpenSession,
		ssSearchItems,
		dbusPropertiesGet,
		ssGetSecret,
	}, f.history())
}

func TestSecretServiceRetriesAStaleSessionOnlyOnce(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	f.addItem(&fakeItem{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}})
	f.configure(func() { f.refuse[ssGetSecret] = ssNoSessionError })
	_, err := ssDoGet(t, f.store(), ssTestAccount)
	var dbusErr *dbus.Error
	c.True(errors.As(err, &dbusErr))
	c.Equal(ssNoSessionError, dbusErr.Name)
	history := f.history()
	c.Equal(2, ssCount(history, ssOpenSession))
	c.Equal(2, ssCount(history, ssGetSecret))
}

// TestSecretServiceDoesNotRerunWhenAnItemVanishes covers a QtDBus-based service, which answers UnknownObject for an
// item that has gone: that is not the service forgetting the session, so the operation must not start over.
func TestSecretServiceDoesNotRerunWhenAnItemVanishes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	f := newFakeSecretService(t)
	attrs := map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}
	f.addItem(&fakeItem{attrs: attrs})
	other := f.addItem(&fakeItem{attrs: maps.Clone(attrs)})
	f.configure(func() {
		f.goneError = dbus.UnknownObject
		f.afterDelete = func() { delete(f.items, other) }
	})
	c.NoError(ssDoDelete(t, f.store(), ssTestAccount), "a Delete that deleted the secret must succeed")
	history := f.history()
	c.Equal(1, ssCount(history, ssOpenSession))
	c.Equal(1, ssCount(history, ssSearchItems))
	c.Equal(0, len(f.itemPaths()))
}

// TestSecretServiceSkipsAVanishedItem covers an item that is removed after SearchItems returned it, as when another
// process deletes the same secret, whether the service then answers NoSuchObject or, as GDBus and QtDBus do for a path
// that has nothing at it, UnknownObject or UnknownMethod: the item must be skipped, and a secret with no item left
// reported missing.
func TestSecretServiceSkipsAVanishedItem(t *testing.T) {
	t.Parallel()
	for _, goneError := range []string{ssNoSuchObjectError, dbus.UnknownObject, dbus.UnknownMethod} {
		c := check.New(t)
		// setup returns a fake holding two items for the account, the first of which is removed once a call of member
		// has been answered, and one for another account.
		setup := func(member string) *fakeSecretService {
			fake := newFakeSecretService(t)
			attrs := map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}
			first := fake.addItem(&fakeItem{attrs: attrs, value: []byte("first")})
			fake.addItem(&fakeItem{attrs: maps.Clone(attrs), value: []byte("second")})
			fake.addItem(&fakeItem{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssOtherAccount}})
			fake.configure(func() {
				fake.goneError = goneError
				fake.after[member] = func() { delete(fake.items, first) }
			})
			return fake
		}

		value, err := ssDoGet(t, setup(ssSearchItems).store(), ssTestAccount)
		c.NoError(err, goneError)
		c.Equal([]byte("second"), value, goneError)

		accounts, err := ssDoAccounts(t, setup(ssSearchItems).store(), ssTestService)
		c.NoError(err, goneError)
		c.Equal([]string{ssTestAccount, ssOtherAccount}, accounts, goneError)

		f := setup(ssSearchItems)
		c.NoError(ssDoDelete(t, f.store(), ssTestAccount), goneError)
		c.Equal(1, len(f.itemPaths()), goneError)

		f = newFakeSecretService(t)
		only := f.addItem(&fakeItem{attrs: map[string]string{ssServiceAttr: ssTestService, ssAccountAttr: ssTestAccount}})
		f.configure(func() {
			f.goneError = goneError
			f.after[dbusPropertiesGet] = func() { delete(f.items, only) }
		})
		_, err = ssDoGet(t, f.store(), ssTestAccount)
		c.True(errors.Is(err, ErrNotFound), goneError)
		c.Equal([]string{ssOpenSession, ssSearchItems, dbusPropertiesGet, ssGetSecret}, f.history(), goneError)
	}
}

func TestMapDBusError(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, err := range []error{
		dbus.Errorf(dbus.ServiceUnknown, "nobody owns it"),
		dbus.Errorf(dbusNameHasNoOwner, "nobody owns it"),
		dbus.Errorf("org.freedesktop.DBus.Error.Spawn.ChildExited", "it would not start"),
		dbus.Errorf("org.freedesktop.DBus.Error.Spawn.ExecFailed", "it would not start"),
		dbus.Errorf("org.freedesktop.DBus.Error.Spawn.ServiceNotFound", "it would not start"),
		dbus.Errorf("org.freedesktop.systemd1.NoSuchUnit", "systemd could not start it"),
		dbus.Errorf("org.freedesktop.systemd1.UnitMasked", "systemd could not start it"),
		dbus.Errorf("org.freedesktop.systemd1.LoadFailed", "systemd could not start it"),
		dbus.Errorf("org.freedesktop.DBus.Error.NoReply", "it went away"),
		dbus.Errorf("org.freedesktop.DBus.Error.TimedOut", "it took too long"),
		dbus.ErrClosed,
		errors.New("dbus: the peer closed the connection"),
	} {
		mapped := mapDBusError(err)
		c.True(errors.Is(mapped, ErrUnavailable), err)
		c.False(errors.Is(mapped, ErrNotFound), err)
	}
	mapped := mapDBusError(dbus.Errorf(ssNoSuchObjectError, "gone"))
	c.True(errors.Is(mapped, ErrNotFound))
	c.False(errors.Is(mapped, ErrUnavailable))
	for _, name := range []string{ssIsLockedError, dbus.UnknownObject, dbus.Failed} {
		mapped = mapDBusError(dbus.Errorf(name, "no"))
		var dbusErr *dbus.Error
		c.True(errors.As(mapped, &dbusErr), name)
		c.Equal(name, dbusErr.Name)
		c.False(errors.Is(mapped, ErrNotFound), name)
		c.False(errors.Is(mapped, ErrUnavailable), name)
	}
}
