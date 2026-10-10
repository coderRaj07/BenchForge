// Package security checks credentials without writing them to logs.
package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
)

// ErrUnauthorized is returned for a missing or wrong credential.
var ErrUnauthorized = errors.New("unauthorized")

// Bearer reports whether the Authorization header carries the expected token.
func Bearer(header, token string) error {
	if token == "" {
		return ErrUnauthorized
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ErrUnauthorized
	}
	got := header[len(prefix):]
	if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
		return ErrUnauthorized
	}
	return nil
}

// GitHubSignature checks X-Hub-Signature-256 against the raw body.
func GitHubSignature(secret string, header string, body []byte) error {
	if secret == "" || !strings.HasPrefix(header, "sha256=") {
		return ErrUnauthorized
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	got := header[len("sha256="):]
	if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		return ErrUnauthorized
	}
	return nil
}
