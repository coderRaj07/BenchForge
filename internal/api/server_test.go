package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coderRaj07/benchforge/internal/scheduler"
)

func TestSubmitRequiresTokenAndIsIdempotent(t *testing.T) {
	svc := &scheduler.Service{Store: scheduler.NewMemory(), MaxGlobal: 2, MaxPerRepo: 2, Timeout: time.Minute}
	srv := &Server{Token: "secret", Svc: svc}
	body := []byte(`{"repository":"acme/orders","manifestPath":"workloads/manifests/orders-select.yaml","workspace":"/repo","idempotencyKey":"k1"}`)
	denied := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, denied)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	again := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	again.Header.Set("Authorization", "Bearer secret")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, again)
	if svc.Store.Depth() != 1 {
		t.Fatalf("depth %d", svc.Store.Depth())
	}
}

func TestWebhookRejectsUnknownRepo(t *testing.T) {
	body := []byte(`{"repository":{"full_name":"other/repo"}}`)
	srv := &Server{WebhookSecret: "topsecret", AllowedRepos: map[string]bool{"acme/orders": true}, Svc: &scheduler.Service{Store: scheduler.NewMemory()}}
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks/github", bytes.NewReader(body))
	mac := hmac.New(sha256.New, []byte("topsecret"))
	mac.Write(body)
	req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}
