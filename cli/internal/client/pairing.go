package client

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/nacl/box"
)

const keypairFilename = "daemon-keypair.json"

// DaemonKeyPair holds the Curve25519 key pair for relay E2EE pairing.
type DaemonKeyPair struct {
	PublicKeyB64 string
	SecretKeyB64 string
}

// LoadOrCreateDaemonKeyPair loads the daemon key pair from ~/.solo/daemon-keypair.json,
// or generates a new one if it doesn't exist.
func LoadOrCreateDaemonKeyPair() (*DaemonKeyPair, error) {
	home := soloHome()
	filePath := filepath.Join(home, keypairFilename)

	// Try loading existing
	data, err := os.ReadFile(filePath)
	if err == nil {
		var stored struct {
			V            int    `json:"v"`
			PublicKeyB64 string `json:"publicKeyB64"`
			SecretKeyB64 string `json:"secretKeyB64"`
		}
		if err := json.Unmarshal(data, &stored); err == nil && stored.V == 2 && stored.PublicKeyB64 != "" {
			// Validate secret key length: old Ed25519 keys are 64 bytes, new Curve25519 are 32.
			secretBytes, _ := base64.StdEncoding.DecodeString(stored.SecretKeyB64)
			if len(secretBytes) == 32 {
				return &DaemonKeyPair{
					PublicKeyB64: stored.PublicKeyB64,
					SecretKeyB64: stored.SecretKeyB64,
				}, nil
			}
			// Old Ed25519 key detected — fall through to regenerate.
		}
	}

	// Generate new Curve25519 key pair (compatible with NaCl box / tweetnacl)
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	kp := &DaemonKeyPair{
		PublicKeyB64: base64.StdEncoding.EncodeToString(pub[:]),
		SecretKeyB64: base64.StdEncoding.EncodeToString(priv[:]),
	}

	// Persist
	if err := os.MkdirAll(home, 0755); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"v":            2,
		"publicKeyB64": kp.PublicKeyB64,
		"secretKeyB64": kp.SecretKeyB64,
	}
	data, _ = json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(filePath, append(data, '\n'), 0600); err != nil {
		return nil, err
	}

	return kp, nil
}

// ConnectionOfferV2 is the relay pairing offer payload.
type ConnectionOfferV2 struct {
	V                  int    `json:"v"`
	ServerID           string `json:"serverId"`
	DaemonPublicKeyB64 string `json:"daemonPublicKeyB64"`
	Relay              struct {
		Endpoint string `json:"endpoint"`
	} `json:"relay"`
}

// GeneratePairingOffer creates a connection offer URL for pairing mobile/desktop apps.
func GeneratePairingOffer(serverID, relayEndpoint, appBaseURL string) (string, error) {
	kp, err := LoadOrCreateDaemonKeyPair()
	if err != nil {
		return "", fmt.Errorf("load daemon key pair: %w", err)
	}

	offer := ConnectionOfferV2{
		V:                  2,
		ServerID:           serverID,
		DaemonPublicKeyB64: kp.PublicKeyB64,
	}
	offer.Relay.Endpoint = relayEndpoint

	jsonData, _ := json.Marshal(offer)
	encoded := base64.RawURLEncoding.EncodeToString(jsonData)

	base := appBaseURL
	if len(base) > 0 && base[len(base)-1] == '/' {
		base = base[:len(base)-1]
	}

	return fmt.Sprintf("%s/#offer=%s", base, encoded), nil
}

// DecodePairingOffer extracts and decodes the ConnectionOfferV2 from a pairing URL.
func DecodePairingOffer(url string) (ConnectionOfferV2, error) {
	var offer ConnectionOfferV2

	// Find the offer fragment
	const prefix = "#offer="
	idx := -1
	for i := 0; i <= len(url)-len(prefix); i++ {
		if url[i:i+len(prefix)] == prefix {
			idx = i + len(prefix)
			break
		}
	}
	if idx == -1 {
		return offer, fmt.Errorf("no offer fragment found in URL")
	}

	encoded := url[idx:]
	jsonData, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return offer, fmt.Errorf("decode base64: %w", err)
	}

	if err := json.Unmarshal(jsonData, &offer); err != nil {
		return offer, fmt.Errorf("unmarshal offer: %w", err)
	}

	return offer, nil
}
