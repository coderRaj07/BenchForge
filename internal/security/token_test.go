package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestBearer(t *testing.T) {
	if err := Bearer("Bearer secret", "secret"); err != nil {
		t.Fatal(err)
	}
	if err := Bearer("Bearer nope", "secret"); err != ErrUnauthorized {
		t.Fatalf("got %v", err)
	}
	if err := Bearer("", "secret"); err != ErrUnauthorized {
		t.Fatal("missing header should fail")
	}
}

func TestGitHubSignature(t *testing.T) {
	body := []byte(`{"zen":"keep it isolated"}`)
	header := SignGitHub("topsecret", body)
	if err := GitHubSignature("topsecret", header, body); err != nil {
		t.Fatal(err)
	}
	if err := GitHubSignature("topsecret", header, []byte("tampered")); err != ErrUnauthorized {
		t.Fatal("tampered body was accepted")
	}
}

func SignGitHub(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
