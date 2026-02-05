// Package crypto provides cryptographic utilities for the shipping aggregation platform.
// It includes AES-256-GCM encryption for BYOA credentials and HMAC-SHA256 signing for webhooks.
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

	"golang.org/x/crypto/hkdf"
)

// AES-256-GCM constants
const (
	// AESKeySize is the required key size for AES-256 (32 bytes)
	AESKeySize = 32
	// AESNonceSize is the standard nonce size for GCM (12 bytes / 96 bits)
	AESNonceSize = 12
)

// Encryption errors
var (
	// ErrInvalidKeySize is returned when the encryption key is not 32 bytes
	ErrInvalidKeySize = errors.New("crypto: invalid key size, must be 32 bytes for AES-256")
	// ErrInvalidCiphertext is returned when the ciphertext is too short or malformed
	ErrInvalidCiphertext = errors.New("crypto: invalid ciphertext")
	// ErrDecryptionFailed is returned when decryption fails (authentication failed)
	ErrDecryptionFailed = errors.New("crypto: decryption failed, authentication error")
)

// AESEncryptor provides AES-256-GCM encryption and decryption operations.
// It is safe for concurrent use.
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor creates a new AES-256-GCM encryptor with the given key.
// The key must be exactly 32 bytes (256 bits).
func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
	if len(key) != AESKeySize {
		return nil, ErrInvalidKeySize
	}

	// Make a copy of the key to prevent external modification
	keyCopy := make([]byte, AESKeySize)
	copy(keyCopy, key)

	return &AESEncryptor{key: keyCopy}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM.
// Returns the ciphertext (nonce prepended) and any error.
// The output format is: nonce (12 bytes) || ciphertext || auth tag (16 bytes)
func (e *AESEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce to ciphertext
	// Seal appends the ciphertext and auth tag to the nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext that was encrypted with Encrypt.
// The ciphertext must include the prepended nonce.
func (e *AESEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	// Verify minimum ciphertext length (nonce + at least auth tag)
	if len(ciphertext) < gcm.NonceSize()+gcm.Overhead() {
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce and actual ciphertext
	nonce := ciphertext[:gcm.NonceSize()]
	encryptedData := ciphertext[gcm.NonceSize():]

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptCredentials encrypts BYOA credentials (as JSON bytes) and returns
// base64-encoded ciphertext suitable for database storage.
func (e *AESEncryptor) EncryptCredentials(credentials []byte) (string, error) {
	ciphertext, err := e.Encrypt(credentials)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptCredentials decrypts base64-encoded ciphertext back to credentials.
func (e *AESEncryptor) DecryptCredentials(encoded string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to decode base64: %w", err)
	}
	return e.Decrypt(ciphertext)
}

// EncryptCredentialsWithIV encrypts credentials and returns ciphertext and IV separately.
// This is useful when you need to store the IV in a separate database column.
// Returns: ciphertext (without nonce), IV (nonce), error
func EncryptCredentialsWithIV(plaintext []byte, key []byte) (ciphertext, iv []byte, err error) {
	if len(key) != AESKeySize {
		return nil, nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	// Generate a random nonce/IV
	iv = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, fmt.Errorf("crypto: failed to generate IV: %w", err)
	}

	// Encrypt (ciphertext includes auth tag)
	ciphertext = gcm.Seal(nil, iv, plaintext, nil)

	return ciphertext, iv, nil
}

// DecryptCredentialsWithIV decrypts credentials using separately provided ciphertext and IV.
func DecryptCredentialsWithIV(ciphertext, iv, key []byte) ([]byte, error) {
	if len(key) != AESKeySize {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: failed to create GCM: %w", err)
	}

	// Verify IV size
	if len(iv) != gcm.NonceSize() {
		return nil, fmt.Errorf("crypto: invalid IV size, expected %d bytes", gcm.NonceSize())
	}

	// Verify minimum ciphertext length (at least auth tag)
	if len(ciphertext) < gcm.Overhead() {
		return nil, ErrInvalidCiphertext
	}

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// GenerateKey generates a cryptographically secure random key for AES-256.
// This should be used for generating new encryption keys.
func GenerateKey() ([]byte, error) {
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("crypto: failed to generate key: %w", err)
	}
	return key, nil
}

// DeriveKeyFromMaster derives an organization-specific key from a master key.
// Uses HKDF (RFC 5869) with SHA-256 for proper cryptographic key derivation.
// The orgID is used as the salt, and a fixed info string provides domain separation.
func DeriveKeyFromMaster(masterKey []byte, orgID []byte) ([]byte, error) {
	if len(masterKey) != AESKeySize {
		return nil, ErrInvalidKeySize
	}

	hkdfReader := hkdf.New(sha256.New, masterKey, orgID, []byte("credential-encryption"))
	key := make([]byte, AESKeySize) // AES-256
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}
	return key, nil
}
