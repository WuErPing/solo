package e2ee

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

const nonceLength = 24

// KeyPair holds a Curve25519 public/secret key pair.
type KeyPair struct {
	PublicKey [32]byte
	SecretKey [32]byte
}

// GenerateKeyPair generates a new Curve25519 key pair.
func GenerateKeyPair() (publicKey, secretKey [32]byte, err error) {
	pub, sec, genErr := box.GenerateKey(rand.Reader)
	if genErr != nil {
		return publicKey, secretKey, fmt.Errorf("generate key pair: %w", genErr)
	}
	return *pub, *sec, nil
}

// ExportPublicKey encodes a public key as standard base64.
func ExportPublicKey(key [32]byte) string {
	return base64.StdEncoding.EncodeToString(key[:])
}

// ImportPublicKey decodes a standard base64 string into a 32-byte public key.
func ImportPublicKey(s string) ([32]byte, error) {
	return importKey(s)
}

// ExportSecretKey encodes a secret key as standard base64.
func ExportSecretKey(key [32]byte) string {
	return base64.StdEncoding.EncodeToString(key[:])
}

// ImportSecretKey decodes a standard base64 string into a 32-byte secret key.
func ImportSecretKey(s string) ([32]byte, error) {
	return importKey(s)
}

func importKey(s string) ([32]byte, error) {
	var key [32]byte
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return key, fmt.Errorf("import key: invalid base64: %w", err)
	}
	if len(b) != 32 {
		return key, errors.New("import key: invalid length (expected 32 bytes)")
	}
	copy(key[:], b)
	return key, nil
}

// DeriveSharedKey computes the shared key from our secret key and the peer's public key
// using Curve25519 ECDH (box.Precompute).
func DeriveSharedKey(secretKey, peerPublicKey [32]byte) [32]byte {
	var shared [32]byte
	box.Precompute(&shared, &peerPublicKey, &secretKey)
	return shared
}

// Encrypt encrypts plaintext using the shared key with a random 24-byte nonce.
// The returned bundle is: [nonce (24 bytes)] [ciphertext...].
func Encrypt(sharedKey [32]byte, plaintext []byte) ([]byte, error) {
	var nonce [nonceLength]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("encrypt: generate nonce: %w", err)
	}
	encrypted := box.SealAfterPrecomputation(nonce[:], plaintext, &nonce, &sharedKey)
	return encrypted, nil
}

// Decrypt decrypts a bundle produced by Encrypt.
// bundle = [nonce (24 bytes)] [ciphertext...]
func Decrypt(sharedKey [32]byte, bundle []byte) ([]byte, error) {
	if len(bundle) < nonceLength {
		return nil, errors.New("decrypt: bundle too short")
	}
	var nonce [nonceLength]byte
	copy(nonce[:], bundle[:nonceLength])
	ciphertext := bundle[nonceLength:]
	plaintext, ok := box.OpenAfterPrecomputation(nil, ciphertext, &nonce, &sharedKey)
	if !ok {
		return nil, errors.New("decrypt: authentication failed")
	}
	return plaintext, nil
}
