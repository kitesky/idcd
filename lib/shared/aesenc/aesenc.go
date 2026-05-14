// Package aesenc provides AES-256-GCM field-level at-rest encryption.
// Wire format: 12-byte random nonce || GCM ciphertext || 16-byte GCM tag.
package aesenc

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// Cipher wraps AES-256-GCM for field-level encryption.
type Cipher struct {
	gcm cipher.AEAD
}

// New creates a Cipher from a 32-byte AES-256 key.
func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("aesenc: key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aesenc: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aesenc: new GCM: %w", err)
	}
	return &Cipher{gcm: gcm}, nil
}

// NewFromHex creates a Cipher from a 64-character hex-encoded 32-byte key.
func NewFromHex(s string) (*Cipher, error) {
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("aesenc: decode hex key: %w", err)
	}
	return New(key)
}

// Encrypt returns nonce (12 B) || GCM ciphertext+tag.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("aesenc: nonce: %w", err)
	}
	return c.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt strips the 12-byte nonce prefix and returns plaintext.
// Returns an error if the data is too short or authentication fails.
func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	ns := c.gcm.NonceSize()
	if len(data) < ns {
		return nil, errors.New("aesenc: ciphertext too short")
	}
	pt, err := c.gcm.Open(nil, data[:ns], data[ns:], nil)
	if err != nil {
		return nil, fmt.Errorf("aesenc: decrypt: %w", err)
	}
	return pt, nil
}
