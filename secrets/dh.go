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
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"math/big"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// The Secret Service's "dh-ietf1024-sha256-aes128-cbc-pkcs7" algorithm: Diffie-Hellman over the 1024-bit MODP group,
// HKDF-SHA256 with no salt and no info to turn the shared secret into an AES-128 key, then AES-128-CBC with PKCS#7
// padding for each secret.
const (
	// dhPrimeHex is the prime of the Second Oakley Group (RFC 2409 §6.2), whose generator is 2.
	dhPrimeHex = "FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DD" +
		"EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED" +
		"EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE65381FFFFFFFFFFFFFFFF"
	// dhPrimeSize is the length of the prime in bytes.
	dhPrimeSize = 128
	// dhMaxAttempts bounds how many random values newDHKeyPair draws before giving up on its source.
	dhMaxAttempts = 64
	// aes128KeySize is the length of the session key in bytes.
	aes128KeySize = 16
)

var (
	dhPrime = func() *big.Int {
		p, ok := new(big.Int).SetString(dhPrimeHex, 16)
		if !ok {
			panic("secrets: the Diffie-Hellman prime does not parse")
		}
		return p
	}()
	dhGenerator  = big.NewInt(2)
	dhLowest     = big.NewInt(2)
	dhPrimeLess2 = new(big.Int).Sub(dhPrime, big.NewInt(2))
)

// newDHKeyPair returns a private exponent drawn from random, in the range [2, p-2], and the public value 2^priv mod p.
// The public value is sent as pub.Bytes(), with no leading zeros, which is what libsecret sends too.
func newDHKeyPair(random io.Reader) (priv, pub *big.Int, err error) {
	buffer := make([]byte, dhPrimeSize)
	for range dhMaxAttempts {
		if _, err = io.ReadFull(random, buffer); err != nil {
			return nil, nil, errs.NewWithCause("unable to generate a Diffie-Hellman key", err)
		}
		priv = new(big.Int).SetBytes(buffer)
		if priv.Cmp(dhLowest) >= 0 && priv.Cmp(dhPrimeLess2) <= 0 {
			return priv, new(big.Int).Exp(dhGenerator, priv, dhPrime), nil
		}
	}
	return nil, nil, errs.New("unable to generate a Diffie-Hellman key: every random value was out of range")
}

// dhSharedSecret returns peerPub^priv mod p as exactly dhPrimeSize bytes. A peer value outside [2, p-2] is rejected,
// since it would make the result predictable.
func dhSharedSecret(priv, peerPub *big.Int) ([]byte, error) {
	if peerPub == nil || peerPub.Cmp(dhLowest) < 0 || peerPub.Cmp(dhPrimeLess2) > 0 {
		return nil, errs.New("the Secret Service's Diffie-Hellman public value is out of range")
	}
	// The shared value is zero-padded to the length of the prime before it goes into HKDF, as libsecret's
	// egg_dh_gen_secret, python-secretstorage (to_bytes(128, 'big')) and gnome-keyring's CKM_DH_PKCS_DERIVE all do.
	// Without the leading zeros, the 1 session in 256 whose value has a leading zero byte would derive a different key.
	return new(big.Int).Exp(peerPub, priv, dhPrime).FillBytes(make([]byte, dhPrimeSize)), nil
}

// deriveAESKey turns a shared secret into the session's AES-128 key with HKDF-SHA256, using no salt and no info.
func deriveAESKey(shared []byte) ([]byte, error) {
	key, err := hkdf.Key(sha256.New, shared, nil, "", aes128KeySize)
	if err != nil {
		return nil, errs.NewWithCause("unable to derive the Secret Service session key", err)
	}
	return key, nil
}

// encryptCBC pads plaintext with PKCS#7, which always adds at least one byte, so empty input becomes one block, and
// encrypts it with AES-CBC under a random IV.
func encryptCBC(key, plaintext []byte) (iv, ciphertext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, errs.NewWithCause("unable to create the Secret Service session cipher", err)
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	ciphertext = make([]byte, len(plaintext)+padding)
	copy(ciphertext, plaintext)
	for i := len(plaintext); i < len(ciphertext); i++ {
		ciphertext[i] = byte(padding)
	}
	nonce := make([]byte, aes.BlockSize)
	if _, err = rand.Read(nonce); err != nil {
		return nil, nil, errs.NewWithCause("unable to generate an initialization vector", err)
	}
	cipher.NewCBCEncrypter(block, nonce).CryptBlocks(ciphertext, ciphertext)
	return nonce, ciphertext, nil
}

// decryptCBC reverses encryptCBC, rejecting an IV that is not one block, ciphertext that is not a whole number of
// blocks and padding that is not well formed.
func decryptCBC(key, iv, ciphertext []byte) ([]byte, error) {
	if len(iv) != aes.BlockSize {
		return nil, errs.Newf("the initialization vector is %d bytes rather than %d", len(iv), aes.BlockSize)
	}
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, errs.Newf("the encrypted secret is %d bytes, which is not a whole number of %d-byte blocks",
			len(ciphertext), aes.BlockSize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errs.NewWithCause("unable to create the Secret Service session cipher", err)
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)
	padding := int(plaintext[len(plaintext)-1])
	if padding == 0 || padding > aes.BlockSize {
		return nil, errs.New("the decrypted secret's padding is not valid")
	}
	end := len(plaintext) - padding
	for _, b := range plaintext[end:] {
		if int(b) != padding {
			return nil, errs.New("the decrypted secret's padding is not valid")
		}
	}
	return plaintext[:end], nil
}
