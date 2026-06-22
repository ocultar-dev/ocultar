package refinery

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"

	"github.com/ocultar-dev/ocultar/vault"
)

// Encrypt encrypts plaintext with AES-256-GCM using the provided key.
// The result is a hex-encoded string prefixed with the nonce.
func Encrypt(plaintext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return hex.EncodeToString(ciphertext), nil
}

// Keep the unexported alias so internal call-sites are unaffected.
func encrypt(plaintext, key []byte) (string, error) { return Encrypt(plaintext, key) }

// Decrypt decrypts a hex-encoded AES-256-GCM ciphertext produced by Encrypt.
func Decrypt(hexCiphertext string, key []byte) ([]byte, error) {
	data, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// DecryptToken resolves an OCULTAR vault token back to its original plaintext.
// It handles two token formats:
//   - Entity tokens: "[PERSON_1]" (numeric suffix) → canonical_name from entity registry
//   - Hash tokens:   "[PERSON_ab3c12ef]" (8-char hex) → AES-decrypted PII from vault
//
// Returns the token unchanged if it is not found in either store (safe fallback).
func DecryptToken(v vault.Provider, masterKey []byte, token string) (string, error) {
	// Fast-path: check the entity registry first for numeric-suffix tokens
	// (e.g. "[PERSON_1]"). These are canonical entity tokens that map directly
	// to a stored canonical name without AES decryption.
	if entityTokenRe.MatchString(token) {
		if name, found := v.GetEntityByToken(token); found {
			return name, nil
		}
		// Not in entity registry — fall through to hash-based lookup below
		// (handles the edge case where a hash happens to look numeric).
	}

	// Standard path: token has an 8-char hex suffix — AES-decrypt from vault.
	type tokenLookup interface {
		GetEncryptedByToken(token string) (string, bool)
	}
	if tl, ok := v.(tokenLookup); ok {
		encryptedPII, found := tl.GetEncryptedByToken(token)
		if !found {
			return token, nil
		}
		plaintext, err := Decrypt(encryptedPII, masterKey)
		if err != nil {
			log.Printf("[ERROR] decrypt error for token %s (key rotation?). Fail-safe: returning unhydrated token: %v", token, err)
			return token, nil
		}
		return string(plaintext), nil
	}
	// Fall back: no reverse-lookup capability — return token as-is (safe)
	return token, nil
}
