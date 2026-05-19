package client

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestDecodePairingOffer(t *testing.T) {
	offer := ConnectionOfferV2{
		V:                  2,
		ServerID:           "test-server-id",
		DaemonPublicKeyB64: "test-public-key",
	}
	offer.Relay.Endpoint = "relay.solo.sh:443"

	jsonData, _ := json.Marshal(offer)
	encoded := base64.RawURLEncoding.EncodeToString(jsonData)
	url := "https://app.solo.sh/#offer=" + encoded

	decoded, err := DecodePairingOffer(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decoded.V != offer.V {
		t.Errorf("expected V=%d, got %d", offer.V, decoded.V)
	}
	if decoded.ServerID != offer.ServerID {
		t.Errorf("expected ServerID=%q, got %q", offer.ServerID, decoded.ServerID)
	}
	if decoded.DaemonPublicKeyB64 != offer.DaemonPublicKeyB64 {
		t.Errorf("expected DaemonPublicKeyB64=%q, got %q", offer.DaemonPublicKeyB64, decoded.DaemonPublicKeyB64)
	}
	if decoded.Relay.Endpoint != offer.Relay.Endpoint {
		t.Errorf("expected Relay.Endpoint=%q, got %q", offer.Relay.Endpoint, decoded.Relay.Endpoint)
	}
}

func TestDecodePairingOffer_InvalidURL(t *testing.T) {
	_, err := DecodePairingOffer("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestDecodePairingOffer_MissingOffer(t *testing.T) {
	_, err := DecodePairingOffer("https://app.solo.sh/#other=xyz")
	if err == nil {
		t.Fatal("expected error for missing offer fragment")
	}
}

func TestDecodePairingOffer_InvalidBase64(t *testing.T) {
	_, err := DecodePairingOffer("https://app.solo.sh/#offer=!!!invalid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}
