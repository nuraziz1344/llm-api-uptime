package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Vault encrypts and decrypts sensitive strings using AES-256-GCM.
type Vault struct {
	aead cipher.AEAD
}

// NewVault creates a Vault from an explicit key string (hex/base64/passphrase)
// or auto-generates/loads a 32-byte key in <dataDir>/.secret.key.
func NewVault(explicitKey, dataDir string) (*Vault, error) {
	var key []byte
	var err error

	if strings.TrimSpace(explicitKey) != "" {
		key = deriveKey(explicitKey)
	} else {
		key, err = loadOrCreateKeyFile(filepath.Join(dataDir, ".secret.key"))
		if err != nil {
			return nil, fmt.Errorf("secret key bootstrap: %w", err)
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	return &Vault{aead: aead}, nil
}

func deriveKey(raw string) []byte {
	if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw)); err == nil && len(decoded) == 32 {
		return decoded
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

func loadOrCreateKeyFile(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		decoded, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
		if decErr == nil && len(decoded) == 32 {
			return decoded, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	if err := os.WriteFile(path, []byte(encoded+"\n"), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (v *Vault) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := v.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (v *Vault) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	nonceSize := v.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, payload := raw[:nonceSize], raw[nonceSize:]
	plain, err := v.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}

// MaskKey returns a safe display hint like "sk-...9f2a" without exposing the secret.
func MaskKey(apiKey string) string {
	k := strings.TrimSpace(apiKey)
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "****"
	}
	return k[:3] + "..." + k[len(k)-4:]
}
