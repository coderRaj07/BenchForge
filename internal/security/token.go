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
	// What: checks an Authorization header against the API token.
	// Used by: internal/api/server.go ServeHTTP on every route except /healthz and the GitHub webhook. Tested in internal/security/token_test.go.
	// Logic: an empty expected token fails closed. prefix is the literal "Bearer ". header must start with that prefix. got is the remainder. ConstantTimeCompare avoids leaking the token through timing. nil means the bytes match.
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
	// What: checks X-Hub-Signature-256 against the raw webhook body.
	// Used by: internal/api/server.go ServeHTTP for POST /v1/webhooks/github. Tested in internal/security/token_test.go.
	// Logic: secret empty or a header that does not start with sha256= fails. mac is HMAC-SHA256 of body using secret. expected is that MAC in hex. got is the header after the sha256= prefix. ConstantTimeCompare returns nil only when they match.
}
