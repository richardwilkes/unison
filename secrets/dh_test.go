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
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestDHPrime verifies that the prime is the 1024-bit safe prime of RFC 2409 §6.2, whose first and last 64 bits are
// all ones by construction.
func TestDHPrime(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(1024, dhPrime.BitLen())
	c.True(dhPrime.ProbablyPrime(20))
	c.True(new(big.Int).Rsh(dhPrime, 1).ProbablyPrime(20), "(p-1)/2 must be prime too")
	raw := dhPrime.Bytes()
	c.Equal(dhPrimeSize, len(raw))
	c.Equal(strings.Repeat("ff", 8), hex.EncodeToString(raw[:8]))
	c.Equal(strings.Repeat("ff", 8), hex.EncodeToString(raw[len(raw)-8:]))
}

func TestDHKeyAgreement(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	privA, pubA, err := newDHKeyPair(rand.Reader)
	c.NoError(err)
	privB, pubB, err := newDHKeyPair(rand.Reader)
	c.NoError(err)
	c.NotEqual(pubA.String(), pubB.String())
	sharedA, err := dhSharedSecret(privA, pubB)
	c.NoError(err)
	sharedB, err := dhSharedSecret(privB, pubA)
	c.NoError(err)
	c.Equal(sharedA, sharedB)
	c.Equal(dhPrimeSize, len(sharedA))
	keyA, err := deriveAESKey(sharedA)
	c.NoError(err)
	keyB, err := deriveAESKey(sharedB)
	c.NoError(err)
	c.Equal(keyA, keyB)

	payload := []byte("agreed")
	iv, ciphertext, err := encryptCBC(keyA, payload)
	c.NoError(err)
	plaintext, err := decryptCBC(keyB, iv, ciphertext)
	c.NoError(err)
	c.Equal(payload, plaintext)
}

// TestDHSharedSecretIsPadded forces shared values far shorter than the prime and verifies that they still come back
// as 128 bytes, zero-padded on the left, which is what the other end of a Secret Service session feeds to HKDF.
func TestDHSharedSecretIsPadded(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		priv int64
		peer int64
		want int64
	}{
		{priv: 1, peer: 2, want: 2},
		{priv: 3, peer: 2, want: 8},
		{priv: 2, peer: 255, want: 65025},
	} {
		shared, err := dhSharedSecret(big.NewInt(one.priv), big.NewInt(one.peer))
		c.NoError(err)
		c.Equal(dhPrimeSize, len(shared))
		c.Equal(one.want, new(big.Int).SetBytes(shared).Int64())
		c.Equal(make([]byte, dhPrimeSize-8), shared[:dhPrimeSize-8])
	}
}

func TestDHRejectsOutOfRangePeerValues(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	priv, _, err := newDHKeyPair(rand.Reader)
	c.NoError(err)
	one := big.NewInt(1)
	for _, peer := range []*big.Int{
		nil,
		big.NewInt(0),
		big.NewInt(1),
		new(big.Int).Sub(dhPrime, one),
		new(big.Int).Set(dhPrime),
		new(big.Int).Add(dhPrime, one),
	} {
		_, err = dhSharedSecret(priv, peer)
		c.HasError(err, "peer value %v", peer)
	}
	for _, peer := range []*big.Int{big.NewInt(2), new(big.Int).Sub(dhPrime, big.NewInt(2))} {
		_, err = dhSharedSecret(priv, peer)
		c.NoError(err, "peer value %v", peer)
	}
}

// TestNewDHKeyPairRetries feeds newDHKeyPair values below and above the permitted range before a valid one, and
// verifies that it draws again until it gets the valid one.
func TestNewDHKeyPairRetries(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	var source bytes.Buffer
	source.Write(make([]byte, dhPrimeSize))                        // 0
	source.Write(append(make([]byte, dhPrimeSize-1), 1))           // 1
	source.Write(bytes.Repeat([]byte{0xff}, dhPrimeSize))          // 2^1024-1, above p-2
	source.Write(new(big.Int).Sub(dhPrime, big.NewInt(1)).Bytes()) // p-1
	source.Write(append(make([]byte, dhPrimeSize-1), 2))           // 2, the smallest permitted value
	source.Write(bytes.Repeat([]byte{0x55}, dhPrimeSize))          // Never reached
	priv, pub, err := newDHKeyPair(&source)
	c.NoError(err)
	c.Equal(int64(2), priv.Int64())
	c.Equal(int64(4), pub.Int64())
	c.Equal(dhPrimeSize, source.Len(), "only the draws up to the valid one should have been consumed")
}

func TestNewDHKeyPairFailures(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	zeros := &zeroReader{}
	_, _, err := newDHKeyPair(zeros)
	c.True(err != nil && strings.Contains(err.Error(), "out of range"),
		"a source that only ever produces zero must be given up on: %v", err)
	c.Equal(dhMaxAttempts, zeros.reads)
	failure := errors.New("no entropy")
	_, _, err = newDHKeyPair(failingReader{err: failure})
	c.True(errors.Is(err, failure))
	_, _, err = newDHKeyPair(bytes.NewReader(make([]byte, dhPrimeSize-1)))
	c.HasError(err, "a short read must fail")
}

// TestDeriveAESKey checks the derivation against HKDF-SHA256 (RFC 5869) computed by hand: with no salt, the extract
// step keys HMAC with a hash-length run of zeros, and with no info, the expand step's first block is HMAC(PRK, 0x01).
func TestDeriveAESKey(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	shared := bytes.Repeat([]byte{0x5a}, dhPrimeSize)
	key, err := deriveAESKey(shared)
	c.NoError(err)
	c.Equal(aes128KeySize, len(key))
	again, err := deriveAESKey(shared)
	c.NoError(err)
	c.Equal(key, again)

	extract := hmac.New(sha256.New, make([]byte, sha256.Size))
	extract.Write(shared)
	expand := hmac.New(sha256.New, extract.Sum(nil))
	expand.Write([]byte{1})
	c.Equal(expand.Sum(nil)[:aes128KeySize], key)

	shared[0] ^= 1
	other, err := deriveAESKey(shared)
	c.NoError(err)
	c.NotEqual(key, other)
}

func TestCBCRoundTrip(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	key := make([]byte, aes128KeySize)
	_, err := rand.Read(key)
	c.NoError(err)
	for _, size := range []int{0, 1, 15, 16, 17, 1000} {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i*7 + 1)
		}
		iv, ciphertext, encryptErr := encryptCBC(key, plaintext)
		c.NoError(encryptErr)
		c.Equal(aes.BlockSize, len(iv))
		c.Equal((size/aes.BlockSize+1)*aes.BlockSize, len(ciphertext), "size %d", size)
		// Compared only from one block up, where a chance match has odds of at most 2^-128; one byte matches 1 in 256.
		if size >= aes.BlockSize {
			c.False(bytes.Equal(ciphertext[:size], plaintext), "size %d", size)
		}
		decrypted, decryptErr := decryptCBC(key, iv, ciphertext)
		c.NoError(decryptErr)
		c.NotNil(decrypted, "size %d", size)
		c.Equal(plaintext, decrypted, "size %d", size)
	}
}

func TestEncryptCBCUsesAFreshIV(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	key := make([]byte, aes128KeySize)
	iv1, ciphertext1, err := encryptCBC(key, []byte("same"))
	c.NoError(err)
	iv2, ciphertext2, err := encryptCBC(key, []byte("same"))
	c.NoError(err)
	c.NotEqual(iv1, iv2)
	c.NotEqual(ciphertext1, ciphertext2)
	_, _, err = encryptCBC(make([]byte, 5), []byte("same"))
	c.HasError(err, "a key of the wrong size must be refused")
}

func TestDecryptCBCRejects(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	key := make([]byte, aes128KeySize)
	_, err := rand.Read(key)
	c.NoError(err)
	iv, ciphertext, err := encryptCBC(key, []byte("a secret of some length"))
	c.NoError(err)

	for _, size := range []int{0, aes.BlockSize - 1, aes.BlockSize + 1} {
		_, err = decryptCBC(key, make([]byte, size), ciphertext)
		c.HasError(err, "an IV of %d bytes", size)
	}
	for _, size := range []int{0, aes.BlockSize - 1, aes.BlockSize + 1, len(ciphertext) - 1} {
		_, err = decryptCBC(key, iv, ciphertext[:size])
		c.HasError(err, "ciphertext of %d bytes", size)
	}

	// The first block of CBC ciphertext decrypts on its own, so encrypting one block and keeping only the first block
	// of the result makes that block's last bytes the padding decryptCBC has to judge.
	padded := func(tail ...byte) (blockIV, block []byte) {
		plaintext := append(bytes.Repeat([]byte{'x'}, aes.BlockSize-len(tail)), tail...)
		blockIV, block, err = encryptCBC(key, plaintext)
		c.NoError(err)
		return blockIV, block[:aes.BlockSize]
	}
	for _, tail := range [][]byte{{0}, {aes.BlockSize + 1}, {0xff}, {2, 1, 3}, {'x', 2}} {
		blockIV, block := padded(tail...)
		_, err = decryptCBC(key, blockIV, block)
		c.HasError(err, "padding ending %v", tail)
	}
	blockIV, block := padded(3, 3, 3)
	plaintext, err := decryptCBC(key, blockIV, block)
	c.NoError(err)
	c.Equal(bytes.Repeat([]byte{'x'}, aes.BlockSize-3), plaintext)
	blockIV, block = padded(bytes.Repeat([]byte{aes.BlockSize}, aes.BlockSize)...)
	plaintext, err = decryptCBC(key, blockIV, block)
	c.NoError(err)
	c.Equal(0, len(plaintext))
}

// zeroReader is a random source that only ever produces zeros. It counts the reads it serves and fails once it has
// served many more than newDHKeyPair may ask for, so that an unbounded retry loop fails the test instead of hanging it.
type zeroReader struct {
	reads int
}

func (r *zeroReader) Read(p []byte) (int, error) {
	r.reads++
	if r.reads > 2*dhMaxAttempts {
		return 0, errors.New("the key pair generator did not give up")
	}
	clear(p)
	return len(p), nil
}

// failingReader is a random source that always fails.
type failingReader struct {
	err error
}

func (r failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
