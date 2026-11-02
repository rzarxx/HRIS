// Package fieldcrypt mengenkripsi kolom sensitif di level aplikasi dengan AES-256-GCM
// (MASTER.md §5.5). Format keluaran: nonce di depan, diikuti ciphertext.
package fieldcrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const keySize = 32

type Cipher struct{ aead cipher.AEAD }

func New(key []byte) (*Cipher, error) {
	if len(key) != keySize {
		return nil, fmt.Errorf("kunci harus %d byte", keySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// LoadOrCreate membaca kunci base64 dari file. Hanya pengembangan yang boleh membuat kunci baru.
func LoadOrCreate(path string, allowCreate bool) (*Cipher, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) && allowCreate {
		key := make([]byte, keySize)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
			return nil, err
		}
		return New(key)
	}
	if err != nil {
		return nil, fmt.Errorf("baca kunci enkripsi: %w", err)
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("kunci enkripsi bukan base64: %w", err)
	}
	return New(key)
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	size := c.aead.NonceSize()
	if len(data) < size {
		return nil, errors.New("ciphertext terlalu pendek")
	}
	return c.aead.Open(nil, data[:size], data[size:], nil)
}
