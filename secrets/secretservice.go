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
	"crypto/rand"
	"errors"
	"math/big"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/unison/internal/dbus"
)

// The parts of the freedesktop Secret Service API (https://specifications.freedesktop.org/secret-service-spec/) that
// the store uses.
const (
	ssDestination                       = "org.freedesktop.secrets"
	ssServicePath       dbus.ObjectPath = "/org/freedesktop/secrets"
	ssDefaultCollection dbus.ObjectPath = "/org/freedesktop/secrets/aliases/default"
	ssServiceIface                      = "org.freedesktop.Secret.Service"
	ssCollectionIface                   = "org.freedesktop.Secret.Collection"
	ssItemIface                         = "org.freedesktop.Secret.Item"
	ssPromptIface                       = "org.freedesktop.Secret.Prompt"
	ssIsLockedError                     = "org.freedesktop.Secret.Error.IsLocked"
	ssNoSessionError                    = "org.freedesktop.Secret.Error.NoSession"
	ssNoSuchObjectError                 = "org.freedesktop.Secret.Error.NoSuchObject"
	ssLabelProp                         = "org.freedesktop.Secret.Item.Label"
	ssAttributesProp                    = "org.freedesktop.Secret.Item.Attributes"
	ssCollectionLabel                   = "org.freedesktop.Secret.Collection.Label"
	ssOpenSession                       = "OpenSession"
	ssReadAlias                         = "ReadAlias"
	ssCreateCollection                  = "CreateCollection"
	ssSearchItems                       = "SearchItems"
	ssUnlock                            = "Unlock"
	ssCreateItem                        = "CreateItem"
	ssGetSecret                         = "GetSecret"
	ssDelete                            = "Delete"
	ssPrompt                            = "Prompt"
	ssCompleted                         = "Completed"
	dhAlgorithm                         = "dh-ietf1024-sha256-aes128-cbc-pkcs7"
	plainAlgorithm                      = "plain"
	ssServiceAttr                       = "service"
	ssAccountAttr                       = "account"
	ssContentType                       = "application/octet-stream"
	ssDefaultAlias                      = "default"
	noPrompt            dbus.ObjectPath = "/"
	noCollection        dbus.ObjectPath = "/"
	dbusPropertiesIface                 = "org.freedesktop.DBus.Properties"
	dbusPropertiesGet                   = "Get"
	dbusNameHasNoOwner                  = "org.freedesktop.DBus.Error.NameHasNoOwner"
	dbusNoReply                         = "org.freedesktop.DBus.Error.NoReply"
	dbusTimedOut                        = "org.freedesktop.DBus.Error.TimedOut"
	dbusSpawnPrefix                     = "org.freedesktop.DBus.Error.Spawn."
	systemdErrorPrefix                  = "org.freedesktop.systemd1."
)

// The bus's own name, path and interface, from which NameOwnerChanged reports a name changing hands, and the match rule
// that asks for those reports about the Secret Service's name.
const (
	dbusBusName                          = "org.freedesktop.DBus"
	dbusBusPath          dbus.ObjectPath = "/org/freedesktop/DBus"
	dbusNameOwnerChanged                 = "NameOwnerChanged"
	ssServiceOwnerRule                   = "type='signal',sender='" + dbusBusName + "',interface='" + dbusBusName +
		"',member='" + dbusNameOwnerChanged + "',arg0='" + ssDestination + "'"
)

// ssCallTimeout bounds the wait for the reply to a method that a service may hold back while it starts up, as when the
// bus has to launch it, or while it asks the user something from within the call rather than through a prompt object.
// It only stops a service that never answers from holding the caller forever.
const ssCallTimeout = 2 * time.Minute

// errPromptDismissed is the cause of the error returned when the user dismisses a Secret Service prompt.
var errPromptDismissed = errors.New("secrets: the prompt was dismissed")

var _ Store = &secretServiceStore{}

// secretServiceStore is a Store backed by the freedesktop Secret Service, as provided by GNOME Keyring, KWallet and
// others. Set creates each secret as an item in the default collection, which it creates first if there is none,
// labeled "service: account" and carrying the attributes service and account, which is what the other operations
// search for. The store is safe for concurrent use: the lock guards only the connection and session state, so a prompt
// waiting on the user holds up nothing else.
type secretServiceStore struct {
	connect func() (*dbus.Conn, error)
	conn    *dbus.Conn
	// session is nil until opened, and dropped when the service no longer knows it, as after the service restarts, so
	// that the next call opens another.
	session *ssSession
	// disconnected is closed by the one OnDisconnect handler registered for conn. Handlers cannot be unregistered, so
	// exactly one is registered per connection.
	disconnected chan struct{}
	mu           sync.Mutex
}

// ssSession is an open Secret Service session, through which secrets travel encrypted with key, or in the clear when
// key is nil.
type ssSession struct {
	path dbus.ObjectPath
	key  []byte
}

// newSecretServiceStore returns a Store that reaches the Secret Service through the connection that connect returns,
// which is dbus.Session in production. Nothing is contacted until the first call.
func newSecretServiceStore(connect func() (*dbus.Conn, error)) *secretServiceStore {
	return &secretServiceStore{connect: connect}
}

// Set implements Store.
func (s *secretServiceStore) Set(service, account string, secret []byte) error {
	return s.withSession(func(session *ssSession) error {
		return s.set(session, service, account, secret)
	})
}

// Get implements Store.
func (s *secretServiceStore) Get(service, account string) ([]byte, error) {
	var secret []byte
	err := s.withSession(func(session *ssSession) error {
		var getErr error
		secret, getErr = s.get(session, service, account)
		return getErr
	})
	if err != nil {
		return nil, err
	}
	return secret, nil
}

// Delete implements Store.
func (s *secretServiceStore) Delete(service, account string) error {
	return s.withSession(func(_ *ssSession) error {
		return s.delete(service, account)
	})
}

// Accounts implements Store.
func (s *secretServiceStore) Accounts(service string) ([]string, error) {
	var accounts []string
	err := s.withSession(func(_ *ssSession) error {
		var accountsErr error
		accounts, accountsErr = s.accounts(service)
		return accountsErr
	})
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

// withSession runs op with the open session. When op fails because the service no longer knows the session, which is
// what a service that has restarted says, the session is dropped and op runs once more with a new one.
func (s *secretServiceStore) withSession(op func(session *ssSession) error) error {
	for attempt := 0; ; attempt++ {
		_, session, err := s.ready()
		if err != nil {
			return err
		}
		if err = op(session); err == nil || attempt != 0 || !isStaleSession(err) {
			return err
		}
		s.dropSession(session)
	}
}

// ready returns the connection and the open session, connecting and opening a session first if need be. A failure to
// reach the session bus is reported as ErrUnavailable.
func (s *secretServiceStore) ready() (*dbus.Conn, *ssSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil || isClosed(s.disconnected) {
		conn, err := s.connect()
		if err != nil {
			return nil, nil, errs.NewWithCause("unable to reach the session bus: "+err.Error(), ErrUnavailable)
		}
		if conn == nil {
			return nil, nil, errs.NewWithCause("there is no session bus", ErrUnavailable)
		}
		if conn != s.conn {
			s.conn = conn
			s.session = nil
			disconnected := make(chan struct{})
			s.disconnected = disconnected
			conn.OnDisconnect(func(error) { close(disconnected) })
		}
		if isClosed(s.disconnected) {
			return nil, nil, errs.NewWithCause("the session bus connection has been lost", ErrUnavailable)
		}
	}
	if s.session == nil {
		session, err := openSession(s.conn)
		if err != nil {
			return nil, nil, err
		}
		s.session = session
	}
	return s.conn, s.session, nil
}

// dropSession forgets session, unless another call has already replaced it.
func (s *secretServiceStore) dropSession(session *ssSession) {
	s.mu.Lock()
	if s.session == session {
		s.session = nil
	}
	s.mu.Unlock()
}

// connection returns the current connection and the channel closed when it is lost.
func (s *secretServiceStore) connection() (conn *dbus.Conn, disconnected <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn, s.disconnected
}

// openSession opens a session that encrypts secrets with a key agreed by Diffie-Hellman. A service that does not
// support that algorithm refuses with NotSupported, in which case a "plain" session is opened instead, as libsecret
// does: the secrets then cross the session bus unencrypted, which only processes of the same user can reach.
func openSession(conn *dbus.Conn) (*ssSession, error) {
	priv, pub, err := newDHKeyPair(rand.Reader)
	if err != nil {
		return nil, err
	}
	reply, err := callOpenSession(conn, dhAlgorithm, dbus.Variant{Value: pub.Bytes(), Sig: "ay"})
	if err != nil {
		var dbusErr *dbus.Error
		if !errors.As(err, &dbusErr) || dbusErr.Name != dbus.NotSupported {
			return nil, err
		}
		if reply, err = callOpenSession(conn, plainAlgorithm, dbus.Variant{Value: "", Sig: "s"}); err != nil {
			return nil, err
		}
		_, path, parseErr := parseOpenSession(reply)
		if parseErr != nil {
			return nil, parseErr
		}
		return &ssSession{path: path}, nil
	}
	output, path, err := parseOpenSession(reply)
	if err != nil {
		return nil, err
	}
	peer, ok := output.Value.([]byte)
	if !ok {
		return nil, errs.Newf("the Secret Service answered the key exchange with %q rather than bytes", output.Sig)
	}
	shared, err := dhSharedSecret(priv, new(big.Int).SetBytes(peer))
	if err != nil {
		return nil, err
	}
	key, err := deriveAESKey(shared)
	if err != nil {
		return nil, err
	}
	return &ssSession{path: path, key: key}, nil
}

func callOpenSession(conn *dbus.Conn, algorithm string, input dbus.Variant) (*dbus.Message, error) {
	msg := dbus.NewMethodCall(ssDestination, ssServicePath, ssServiceIface, ssOpenSession)
	if err := msg.SetBodyWithSignature("sv", algorithm, input); err != nil {
		return nil, errs.Wrap(err)
	}
	reply, err := conn.CallWithTimeout(msg, 0, ssCallTimeout)
	if err != nil {
		return nil, mapDBusError(err)
	}
	return reply, nil
}

func parseOpenSession(reply *dbus.Message) (output dbus.Variant, path dbus.ObjectPath, err error) {
	args, err := replyArgs(reply, "vo")
	if err != nil {
		return dbus.Variant{}, "", err
	}
	if output, err = replyArg[dbus.Variant](args, 0); err != nil {
		return dbus.Variant{}, "", err
	}
	if path, err = replyArg[dbus.ObjectPath](args, 1); err != nil {
		return dbus.Variant{}, "", err
	}
	return output, path, nil
}

// seal returns secret as the Secret Service's (oayays) Secret structure: the session, the IV, the value and its
// content type. A plain session sends an empty IV and the secret as it is.
func (s *ssSession) seal(secret []byte) (dbus.Struct, error) {
	if s.key == nil {
		return dbus.Struct{s.path, []byte{}, clone(secret), ssContentType}, nil
	}
	iv, ciphertext, err := encryptCBC(s.key, secret)
	if err != nil {
		return nil, err
	}
	return dbus.Struct{s.path, iv, ciphertext, ssContentType}, nil
}

// open returns the secret held by v, a Secret structure the service sent through this session.
func (s *ssSession) open(v any) ([]byte, error) {
	fields, ok := v.(dbus.Struct)
	if !ok || len(fields) != 4 {
		return nil, errs.Newf("the Secret Service sent a %T rather than a secret", v)
	}
	iv, ivOK := fields[1].([]byte)
	value, valueOK := fields[2].([]byte)
	if !ivOK || !valueOK {
		return nil, errs.New("the Secret Service sent a malformed secret")
	}
	if s.key == nil {
		return clone(value), nil
	}
	return decryptCBC(s.key, iv, value)
}

// call sends msg on the current connection, waits for the reply as long as [dbus.Conn.Call] does and maps a failure
// with mapDBusError. It is for the methods a service answers at once; see callPatiently for the others.
func (s *secretServiceStore) call(msg *dbus.Message) (*dbus.Message, error) {
	conn, _ := s.connection()
	if conn == nil {
		return nil, errs.NewWithCause("there is no session bus connection", ErrUnavailable)
	}
	reply, err := conn.Call(msg)
	if err != nil {
		return nil, mapDBusError(err)
	}
	return reply, nil
}

// callPatiently is call for a method that a service may hold back the reply to, waiting up to ssCallTimeout for it.
func (s *secretServiceStore) callPatiently(msg *dbus.Message) (*dbus.Message, error) {
	conn, _ := s.connection()
	if conn == nil {
		return nil, errs.NewWithCause("there is no session bus connection", ErrUnavailable)
	}
	reply, err := conn.CallWithTimeout(msg, 0, ssCallTimeout)
	if err != nil {
		return nil, mapDBusError(err)
	}
	return reply, nil
}

// unlock asks the service to unlock paths, which are collections or items, and returns the ones that are now unlocked.
// The user may have to be prompted for that, in which case this waits for as long as they take to answer. When the user
// dismisses the prompt, the ones the service unlocked without it are returned with an error matching
// errPromptDismissed.
func (s *secretServiceStore) unlock(paths []dbus.ObjectPath) ([]dbus.ObjectPath, error) {
	msg := dbus.NewMethodCall(ssDestination, ssServicePath, ssServiceIface, ssUnlock)
	if err := msg.SetBodyWithSignature("ao", paths); err != nil {
		return nil, errs.Wrap(err)
	}
	reply, err := s.callPatiently(msg)
	if err != nil {
		return nil, err
	}
	args, err := replyArgs(reply, "aoo")
	if err != nil {
		return nil, err
	}
	unlocked, err := replyArg[[]dbus.ObjectPath](args, 0)
	if err != nil {
		return nil, err
	}
	prompt, err := replyArg[dbus.ObjectPath](args, 1)
	if err != nil {
		return nil, err
	}
	if prompt == noPrompt {
		return unlocked, nil
	}
	result, err := s.runPrompt(prompt, reply.Sender)
	if err != nil {
		if errors.Is(err, errPromptDismissed) {
			return unlocked, err
		}
		return nil, err
	}
	if more, ok := result.Value.([]dbus.ObjectPath); ok {
		unlocked = append(unlocked, more...)
	}
	return unlocked, nil
}

// runPrompt shows the prompt and waits for the user to answer it, for however long that takes, returning the
// prompt's result. owner is the sender of the reply that handed out the prompt: a match rule governs only what the bus
// broadcasts, and anyone on the bus can send this connection a Completed signal directly, so only one from the service
// itself is believed. An empty owner, as on a connection with no bus behind it, accepts any sender. A service that
// quits or is replaced while the prompt is showing can never complete it, so the bus reporting that the service's name
// has lost its owner, or has passed from owner to another, ends the wait with an error matching ErrUnavailable, as
// losing the connection does. A prompt that has already gone when it is to be shown is an error that matches neither
// ErrNotFound nor isMissingObject, since it says nothing about the object the operation is about.
func (s *secretServiceStore) runPrompt(prompt dbus.ObjectPath, owner string) (dbus.Variant, error) {
	conn, disconnected := s.connection()
	if conn == nil {
		return dbus.Variant{}, errs.NewWithCause("there is no session bus connection", ErrUnavailable)
	}
	completed := make(chan *dbus.Message, 1)
	cancel := conn.Subscribe(dbus.SignalFilter{Path: prompt, Interface: ssPromptIface, Member: ssCompleted},
		func(msg *dbus.Message) {
			if owner != "" && msg.Sender != owner {
				return
			}
			select { // Handlers run on the connection's dispatcher, so this must never block
			case completed <- msg:
			default:
			}
		})
	defer cancel()
	rule := "type='signal',interface='" + ssPromptIface + "',member='" + ssCompleted + "',path='" +
		string(prompt) + "'"
	if err := conn.AddMatch(rule); err != nil {
		return dbus.Variant{}, mapDBusError(err)
	}
	defer removeMatch(conn, rule)
	gone := make(chan struct{}, 1)
	cancelOwner := conn.Subscribe(dbus.SignalFilter{
		Sender:    dbusBusName,
		Path:      dbusBusPath,
		Interface: dbusBusName,
		Member:    dbusNameOwnerChanged,
	}, func(msg *dbus.Message) {
		if serviceLeft(msg, owner) {
			select {
			case gone <- struct{}{}:
			default:
			}
		}
	})
	defer cancelOwner()
	if err := conn.AddMatch(ssServiceOwnerRule); err != nil {
		return dbus.Variant{}, mapDBusError(err)
	}
	defer removeMatch(conn, ssServiceOwnerRule)
	msg := dbus.NewMethodCall(ssDestination, prompt, ssPromptIface, ssPrompt)
	if err := msg.SetBodyWithSignature("s", ""); err != nil {
		return dbus.Variant{}, errs.Wrap(err)
	}
	if _, err := s.callPatiently(msg); err != nil {
		if isMissingObject(err) {
			// It is the prompt that has gone, not the item or collection the operation is about, so the error must not
			// read as a missing object, which callers take to mean the latter. A new error carries the text alone.
			return dbus.Variant{}, errs.Newf("the Secret Service dropped the prompt %s before it was shown: %s", prompt,
				err)
		}
		return dbus.Variant{}, err
	}
	select {
	case signal := <-completed:
		return promptResult(signal)
	case <-gone:
		select { // The answer may have arrived just before the service went
		case signal := <-completed:
			return promptResult(signal)
		default:
		}
		return dbus.Variant{}, errs.NewWithCause("the Secret Service went away during a prompt", ErrUnavailable)
	case <-disconnected:
		select { // The answer may have arrived just before the connection was lost
		case signal := <-completed:
			return promptResult(signal)
		default:
		}
		return dbus.Variant{}, errs.NewWithCause("the session bus connection was lost during a Secret Service prompt",
			ErrUnavailable)
	}
}

// removeMatch asks the bus to stop delivering the signals a prompt asked for with rule. Only an error reply from the
// bus is logged, not a failure to reach it, which is expected once the connection has been lost.
func removeMatch(conn *dbus.Conn, rule string) {
	if err := conn.RemoveMatch(rule); err != nil && !errors.Is(mapDBusError(err), ErrUnavailable) {
		errs.Log(errs.NewWithCause("unable to remove the match rule for a Secret Service prompt", err), "rule", rule)
	}
}

// serviceLeft reports whether a NameOwnerChanged signal says that the Secret Service's name has lost its owner, or has
// passed from owner to another connection.
func serviceLeft(msg *dbus.Message, owner string) bool {
	args, err := msg.Args()
	if err != nil || len(args) != 3 {
		return false
	}
	name, nameOK := args[0].(string)
	newOwner, newOwnerOK := args[2].(string)
	return nameOK && newOwnerOK && name == ssDestination && (newOwner == "" || (owner != "" && newOwner != owner))
}

// promptResult returns the result a prompt's Completed signal carries, or an error matching errPromptDismissed when the
// user dismissed it.
func promptResult(signal *dbus.Message) (dbus.Variant, error) {
	args, err := replyArgs(signal, "bv")
	if err != nil {
		return dbus.Variant{}, err
	}
	dismissed, err := replyArg[bool](args, 0)
	if err != nil {
		return dbus.Variant{}, err
	}
	if dismissed {
		return dbus.Variant{}, errs.NewWithCause("the Secret Service prompt was dismissed", errPromptDismissed)
	}
	return replyArg[dbus.Variant](args, 1)
}

// search returns the items whose attributes include attrs, split into the unlocked and the locked.
func (s *secretServiceStore) search(attrs map[string]string) (unlocked, locked []dbus.ObjectPath, err error) {
	msg := dbus.NewMethodCall(ssDestination, ssServicePath, ssServiceIface, ssSearchItems)
	if err = msg.SetBodyWithSignature("a{ss}", attrs); err != nil {
		return nil, nil, errs.Wrap(err)
	}
	reply, err := s.callPatiently(msg)
	if err != nil {
		return nil, nil, err
	}
	args, err := replyArgs(reply, "aoao")
	if err != nil {
		return nil, nil, err
	}
	if unlocked, err = replyArg[[]dbus.ObjectPath](args, 0); err != nil {
		return nil, nil, err
	}
	if locked, err = replyArg[[]dbus.ObjectPath](args, 1); err != nil {
		return nil, nil, err
	}
	return unlocked, locked, nil
}

// itemAttributes returns the attributes of the item.
func (s *secretServiceStore) itemAttributes(item dbus.ObjectPath) (map[string]string, error) {
	msg := dbus.NewMethodCall(ssDestination, item, dbusPropertiesIface, dbusPropertiesGet)
	if err := msg.SetBodyWithSignature("ss", ssItemIface, "Attributes"); err != nil {
		return nil, errs.Wrap(err)
	}
	reply, err := s.call(msg)
	if err != nil {
		return nil, err
	}
	args, err := replyArgs(reply, "v")
	if err != nil {
		return nil, err
	}
	variant, err := replyArg[dbus.Variant](args, 0)
	if err != nil {
		return nil, err
	}
	dict, ok := variant.Value.(dbus.Dict)
	if !ok {
		return nil, errs.Newf("the Secret Service reported attributes of type %q", variant.Sig)
	}
	attrs := make(map[string]string, len(dict))
	for _, entry := range dict {
		key, keyOK := entry.Key.(string)
		value, valueOK := entry.Value.(string)
		if keyOK && valueOK {
			attrs[key] = value
		}
	}
	return attrs, nil
}

// unlockItems asks the service to unlock items and returns the ones that are now unlocked, in the order given, and
// whether any were left locked. When the user dismisses the prompt, the ones unlocked without it are returned with an
// error matching errPromptDismissed.
func (s *secretServiceStore) unlockItems(items []dbus.ObjectPath) (opened []dbus.ObjectPath, leftLocked bool,
	err error,
) {
	var unlocked []dbus.ObjectPath
	if unlocked, err = s.unlock(items); err != nil && !errors.Is(err, errPromptDismissed) {
		return nil, false, err
	}
	for _, item := range items {
		if slices.Contains(unlocked, item) {
			opened = append(opened, item)
		} else {
			leftLocked = true
		}
	}
	return opened, leftLocked, err
}

// itemsFor returns the items whose attributes name the service and account, in the order given: a search is only as
// precise as the service that runs it, so what it found is checked before anything is read or deleted. An item that
// has gone is skipped. The items must be unlocked, since GNOME Keyring keeps only hashes of a locked item's attribute
// values.
func (s *secretServiceStore) itemsFor(items []dbus.ObjectPath, service, account string) ([]dbus.ObjectPath, error) {
	var matched []dbus.ObjectPath
	for _, item := range items {
		attrs, err := s.itemAttributes(item)
		if err != nil {
			if isMissingObject(err) {
				continue
			}
			return nil, err
		}
		if attrs[ssServiceAttr] == service && attrs[ssAccountAttr] == account {
			matched = append(matched, item)
		}
	}
	return matched, nil
}

func (s *secretServiceStore) set(session *ssSession, service, account string, secret []byte) error {
	// A service with no default collection may refuse to unlock the alias rather than skip it, so that is left for
	// CreateItem to confirm, which leads to the collection being created below.
	if _, err := s.unlock([]dbus.ObjectPath{ssDefaultCollection}); err != nil && !isMissingObject(err) {
		return err
	}
	sealed, err := session.seal(secret)
	if err != nil {
		return err
	}
	props := dbus.Dict{
		{Key: ssLabelProp, Value: dbus.Variant{Value: service + ": " + account, Sig: "s"}},
		{Key: ssAttributesProp, Value: dbus.Variant{Value: lookupAttributes(service, account), Sig: "a{ss}"}},
	}
	createItem := func() (*dbus.Message, error) {
		msg := dbus.NewMethodCall(ssDestination, ssDefaultCollection, ssCollectionIface, ssCreateItem)
		if bodyErr := msg.SetBodyWithSignature("a{sv}(oayays)b", props, sealed, true); bodyErr != nil {
			return nil, errs.Wrap(bodyErr)
		}
		return s.callPatiently(msg)
	}
	reply, err := createItem()
	if err != nil && isMissingObject(err) {
		// No collection has the default alias, as on a GNOME Keyring that was started without PAM, so one is created
		// and the item tried once more, as libsecret does.
		if err = s.createDefaultCollection(); err != nil {
			return err
		}
		reply, err = createItem()
	}
	if err != nil {
		return err
	}
	args, err := replyArgs(reply, "oo")
	if err != nil {
		return err
	}
	prompt, err := replyArg[dbus.ObjectPath](args, 1)
	if err != nil || prompt == noPrompt {
		return err
	}
	_, err = s.runPrompt(prompt, reply.Sender)
	return err
}

// createDefaultCollection creates a collection with the default alias, prompting the user if the service asks to.
func (s *secretServiceStore) createDefaultCollection() error {
	msg := dbus.NewMethodCall(ssDestination, ssServicePath, ssServiceIface, ssCreateCollection)
	props := dbus.Dict{{Key: ssCollectionLabel, Value: dbus.Variant{Value: "Default keyring", Sig: "s"}}}
	if err := msg.SetBodyWithSignature("a{sv}s", props, ssDefaultAlias); err != nil {
		return errs.Wrap(err)
	}
	reply, err := s.callPatiently(msg)
	if err != nil {
		return err
	}
	args, err := replyArgs(reply, "oo")
	if err != nil {
		return err
	}
	prompt, err := replyArg[dbus.ObjectPath](args, 1)
	if err != nil || prompt == noPrompt {
		return err
	}
	_, err = s.runPrompt(prompt, reply.Sender)
	return err
}

func (s *secretServiceStore) get(session *ssSession, service, account string) ([]byte, error) {
	unlocked, locked, err := s.search(lookupAttributes(service, account))
	if err != nil {
		return nil, err
	}
	item, err := s.pickItem(unlocked, locked, service, account)
	if err != nil {
		return nil, err
	}
	msg := dbus.NewMethodCall(ssDestination, item, ssItemIface, ssGetSecret)
	if err = msg.SetBodyWithSignature("o", session.path); err != nil {
		return nil, errs.Wrap(err)
	}
	reply, err := s.callPatiently(msg)
	if err != nil {
		if isMissingObject(err) {
			return nil, ErrNotFound // The item has gone since it was found
		}
		return nil, err
	}
	args, err := replyArgs(reply, "(oayays)")
	if err != nil {
		return nil, err
	}
	return session.open(args[0])
}

// pickItem returns the item that get reads out of what a search found: one whose attributes name the service and
// account. When there is more than one, as after the user makes another collection the default between two calls of
// Set, one in the collection the default alias names is preferred, since that is the one Set replaces; otherwise the
// first is. A locked item is unlocked only when no unlocked one will do, those in the default collection first. When
// the user dismisses a prompt to unlock, the items the service unlocked without it are still checked, and then the
// dismissal is returned.
func (s *secretServiceStore) pickItem(unlocked, locked []dbus.ObjectPath,
	service, account string,
) (dbus.ObjectPath, error) {
	var preferred dbus.ObjectPath
	if len(unlocked)+len(locked) > 1 {
		var err error
		if preferred, err = s.defaultCollection(); err != nil {
			return "", err
		}
	}
	unlockedIn, unlockedOut := splitByCollection(unlocked, preferred)
	lockedIn, lockedOut := splitByCollection(locked, preferred)
	leftLocked := false
	for _, group := range []struct {
		items  []dbus.ObjectPath
		locked bool
	}{{unlockedIn, false}, {lockedIn, true}, {unlockedOut, false}, {lockedOut, true}} {
		items := group.items
		var unlockErr error
		if group.locked && len(items) != 0 {
			var stillLocked bool
			items, stillLocked, unlockErr = s.unlockItems(items)
			if unlockErr != nil && !errors.Is(unlockErr, errPromptDismissed) {
				return "", unlockErr
			}
			leftLocked = leftLocked || stillLocked
		}
		matched, err := s.itemsFor(items, service, account)
		if err != nil {
			return "", err
		}
		if len(matched) != 0 {
			return matched[0], nil
		}
		if unlockErr != nil {
			return "", unlockErr
		}
	}
	if leftLocked {
		return "", errs.New("the Secret Service left the secret locked")
	}
	return "", ErrNotFound
}

// defaultCollection returns the collection the default alias names, or an empty path when there is none.
func (s *secretServiceStore) defaultCollection() (dbus.ObjectPath, error) {
	msg := dbus.NewMethodCall(ssDestination, ssServicePath, ssServiceIface, ssReadAlias)
	if err := msg.SetBodyWithSignature("s", ssDefaultAlias); err != nil {
		return "", errs.Wrap(err)
	}
	reply, err := s.call(msg)
	if err != nil {
		return "", err
	}
	args, err := replyArgs(reply, "o")
	if err != nil {
		return "", err
	}
	collection, err := replyArg[dbus.ObjectPath](args, 0)
	if err != nil || collection == noCollection {
		return "", err
	}
	return collection, nil
}

// splitByCollection splits items into those in the collection and the rest, keeping their order. The Secret Service
// API gives an item the path of its collection followed by one more element.
func splitByCollection(items []dbus.ObjectPath, collection dbus.ObjectPath) (in, out []dbus.ObjectPath) {
	for _, item := range items {
		if i := strings.LastIndexByte(string(item), '/'); collection != "" && i > 0 && item[:i] == collection {
			in = append(in, item)
		} else {
			out = append(out, item)
		}
	}
	return in, out
}

// delete removes every item for the service and account, unlocking the locked ones first. An item that has gone by
// the time it is deleted is skipped, as is one whose attributes name another service or account. An item the service
// leaves locked cannot be checked, so it is not deleted, and the error returned says so.
func (s *secretServiceStore) delete(service, account string) error {
	unlocked, locked, err := s.search(lookupAttributes(service, account))
	if err != nil {
		return err
	}
	if len(unlocked) == 0 && len(locked) == 0 {
		return ErrNotFound
	}
	leftLocked := false
	if len(locked) != 0 {
		var opened []dbus.ObjectPath
		if opened, leftLocked, err = s.unlockItems(locked); err != nil {
			return err
		}
		unlocked = append(unlocked, opened...)
	}
	items, err := s.itemsFor(unlocked, service, account)
	if err != nil {
		return err
	}
	deleted := 0
	for _, item := range items {
		if err = s.deleteItem(item); err != nil {
			if isMissingObject(err) {
				continue
			}
			return err
		}
		deleted++
	}
	if leftLocked {
		return errs.New("the Secret Service left the secret locked")
	}
	if deleted == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *secretServiceStore) deleteItem(item dbus.ObjectPath) error {
	reply, err := s.callPatiently(dbus.NewMethodCall(ssDestination, item, ssItemIface, ssDelete))
	if err != nil {
		return err
	}
	args, err := replyArgs(reply, "o")
	if err != nil {
		return err
	}
	prompt, err := replyArg[dbus.ObjectPath](args, 0)
	if err != nil || prompt == noPrompt {
		return err
	}
	_, err = s.runPrompt(prompt, reply.Sender)
	return err
}

// accounts reads the account of every item that matches the service, unlocking the locked ones first: GNOME Keyring
// keeps only hashes of a locked item's attribute values, enough for a search to match on but not to read, so the user
// may be prompted, as Get would prompt. An item still locked afterwards, as when the user dismisses the prompt, is
// skipped, as is one whose attributes name another service, since a search is only as precise as the service that
// runs it.
func (s *secretServiceStore) accounts(service string) ([]string, error) {
	unlocked, locked, err := s.search(map[string]string{ssServiceAttr: service})
	if err != nil {
		return nil, err
	}
	if len(locked) != 0 {
		var opened []dbus.ObjectPath
		if opened, err = s.unlock(locked); err != nil && !errors.Is(err, errPromptDismissed) {
			return nil, err
		}
		for _, item := range locked {
			if slices.Contains(opened, item) {
				unlocked = append(unlocked, item)
			}
		}
	}
	accounts := make([]string, 0, len(unlocked))
	for _, item := range unlocked {
		attrs, attrsErr := s.itemAttributes(item)
		if attrsErr != nil {
			if isMissingObject(attrsErr) {
				continue
			}
			return nil, attrsErr
		}
		if account := attrs[ssAccountAttr]; account != "" && attrs[ssServiceAttr] == service {
			accounts = append(accounts, account)
		}
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	slices.Sort(accounts)
	return slices.Compact(accounts), nil
}

// lookupAttributes returns the attributes that identify the item for the service and account.
func lookupAttributes(service, account string) map[string]string {
	return map[string]string{ssServiceAttr: service, ssAccountAttr: account}
}

// mapDBusError turns the failure of a call into the error the store reports. A failure that is not an error reply
// means the bus connection failed or nothing answered, so it matches ErrUnavailable, as does a bus that knows of no
// Secret Service, cannot start it or says that it did not answer. A bus that has systemd start the service passes on
// systemd's own error when that fails. NoSuchObject matches ErrNotFound. Any other error reply is wrapped, so that
// errors.As still finds the *dbus.Error.
func mapDBusError(err error) error {
	var dbusErr *dbus.Error
	if !errors.As(err, &dbusErr) {
		return errs.NewWithCause("unable to reach the Secret Service: "+err.Error(), ErrUnavailable)
	}
	if strings.HasPrefix(dbusErr.Name, dbusSpawnPrefix) || strings.HasPrefix(dbusErr.Name, systemdErrorPrefix) {
		return errs.NewWithCause("the Secret Service could not be started: "+dbusErr.Error(), ErrUnavailable)
	}
	switch dbusErr.Name {
	case dbus.ServiceUnknown, dbusNameHasNoOwner:
		return errs.NewWithCause("nothing provides the Secret Service: "+dbusErr.Error(), ErrUnavailable)
	case dbusNoReply, dbusTimedOut:
		return errs.NewWithCause("the Secret Service did not answer: "+dbusErr.Error(), ErrUnavailable)
	case ssNoSuchObjectError:
		return errs.NewWithCause("the Secret Service has no such object: "+dbusErr.Error(), ErrNotFound)
	case ssIsLockedError:
		return errs.NewWithCause("the Secret Service collection is locked", dbusErr)
	default:
		return errs.NewWithCause("the Secret Service failed", dbusErr)
	}
}

// isMissingObject reports whether err says that the object a call was addressed to does not exist: NoSuchObject from
// the service itself, or what GDBus and QtDBus answer for a path that has nothing at it, which libsecret accepts too.
func isMissingObject(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var dbusErr *dbus.Error
	return errors.As(err, &dbusErr) && (dbusErr.Name == dbus.UnknownObject || dbusErr.Name == dbus.UnknownMethod)
}

// isStaleSession reports whether err says the service does not know the session, which is what a service that has
// restarted since the session was opened says. UnknownObject is not taken to mean that, since it is also what a
// QtDBus-based service answers for an item that has gone.
func isStaleSession(err error) bool {
	var dbusErr *dbus.Error
	return errors.As(err, &dbusErr) && dbusErr.Name == ssNoSessionError
}

// replyArgs returns the arguments of msg after checking that they have the signature sig.
func replyArgs(msg *dbus.Message, sig dbus.Signature) ([]any, error) {
	if msg.Signature != sig {
		return nil, errs.Newf("the Secret Service sent (%s) where (%s) was expected", msg.Signature, sig)
	}
	args, err := msg.Args()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return args, nil
}

// replyArg returns args[i] as a T.
func replyArg[T any](args []any, i int) (T, error) {
	var zero T
	if i >= len(args) {
		return zero, errs.Newf("the Secret Service sent %d values where at least %d were expected", len(args), i+1)
	}
	v, ok := args[i].(T)
	if !ok {
		return zero, errs.Newf("the Secret Service sent a %T where a %T was expected", args[i], zero)
	}
	return v, nil
}

// isClosed reports whether ch has been closed, without waiting.
func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
