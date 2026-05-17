package e2ee_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/WuErPing/solo/relay/internal/e2ee"
)

func TestGenerateKeyPair(t *testing.T) {
	pub, sec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error: %v", err)
	}
	var zero [32]byte
	if pub == zero {
		t.Error("public key is all zeros")
	}
	if sec == zero {
		t.Error("secret key is all zeros")
	}
}

func TestExportImportPublicKey(t *testing.T) {
	pub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	exported := e2ee.ExportPublicKey(pub)
	if exported == "" {
		t.Fatal("ExportPublicKey returned empty string")
	}
	imported, err := e2ee.ImportPublicKey(exported)
	if err != nil {
		t.Fatalf("ImportPublicKey error: %v", err)
	}
	if imported != pub {
		t.Error("imported public key does not match original")
	}
	// re-export should match
	if e2ee.ExportPublicKey(imported) != exported {
		t.Error("re-exported key does not match original export")
	}
}

func TestExportImportSecretKey(t *testing.T) {
	_, sec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	exported := e2ee.ExportSecretKey(sec)
	if exported == "" {
		t.Fatal("ExportSecretKey returned empty string")
	}
	imported, err := e2ee.ImportSecretKey(exported)
	if err != nil {
		t.Fatalf("ImportSecretKey error: %v", err)
	}
	if imported != sec {
		t.Error("imported secret key does not match original")
	}
}

func TestImportPublicKeyRejectsInvalidBase64(t *testing.T) {
	_, err := e2ee.ImportPublicKey("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestImportPublicKeyRejectsWrongLength(t *testing.T) {
	// 16 bytes base64-encoded — wrong length
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err := e2ee.ImportPublicKey(short)
	if err == nil {
		t.Error("expected error for wrong-length key, got nil")
	}
}

func TestImportSecretKeyRejectsInvalidBase64(t *testing.T) {
	_, err := e2ee.ImportSecretKey("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestImportSecretKeyRejectsWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	_, err := e2ee.ImportSecretKey(short)
	if err == nil {
		t.Error("expected error for wrong-length key, got nil")
	}
}

func TestDeriveSharedKeyBothSides(t *testing.T) {
	// ECDH property: both sides derive the same shared key
	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	clientPub, clientSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	daemonShared := e2ee.DeriveSharedKey(daemonSec, clientPub)
	clientShared := e2ee.DeriveSharedKey(clientSec, daemonPub)

	if daemonShared != clientShared {
		t.Error("daemon and client derived different shared keys")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	daemonPub, daemonSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	_, clientSec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sharedKey := e2ee.DeriveSharedKey(clientSec, daemonPub)
	_ = daemonSec

	plaintext := []byte("Hello, encrypted world! 你好世界")
	ciphertext, err := e2ee.Encrypt(sharedKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	decrypted, err := e2ee.Decrypt(sharedKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted %q != plaintext %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	_, sec1, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	pub2, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	pub3, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	correctKey := e2ee.DeriveSharedKey(sec1, pub2)
	wrongKey := e2ee.DeriveSharedKey(sec1, pub3)

	ct, err := e2ee.Encrypt(correctKey, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = e2ee.Decrypt(wrongKey, ct)
	if err == nil {
		t.Error("expected decryption to fail with wrong key, got nil error")
	}
}

func TestEncryptProducesDifferentCiphertextEachTime(t *testing.T) {
	_, sec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sharedKey := e2ee.DeriveSharedKey(sec, pub)

	plaintext := []byte("Same message")
	ct1, err := e2ee.Encrypt(sharedKey, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	ct2, err := e2ee.Encrypt(sharedKey, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(ct1, ct2) {
		t.Error("expected different ciphertexts due to random nonce, got identical")
	}
	// both should decrypt correctly
	d1, err := e2ee.Decrypt(sharedKey, ct1)
	if err != nil {
		t.Fatalf("Decrypt ct1: %v", err)
	}
	d2, err := e2ee.Decrypt(sharedKey, ct2)
	if err != nil {
		t.Fatalf("Decrypt ct2: %v", err)
	}
	if !bytes.Equal(d1, plaintext) || !bytes.Equal(d2, plaintext) {
		t.Error("decrypted values don't match plaintext")
	}
}

func TestDecryptTooShortBundleFails(t *testing.T) {
	_, sec, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	pub, _, err := e2ee.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	sharedKey := e2ee.DeriveSharedKey(sec, pub)

	_, err = e2ee.Decrypt(sharedKey, []byte("short"))
	if err == nil {
		t.Error("expected error for too-short bundle, got nil")
	}
}
