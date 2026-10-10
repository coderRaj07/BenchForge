// Package api is the HTTP surface for submitting and inspecting benchmark runs.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/coderRaj07/benchforge/internal/scheduler"
	"github.com/coderRaj07/benchforge/internal/security"
)

// Server admits runs. The token is never written to a response or log line.
type Server struct {
	Token         string
	WebhookSecret string
	AllowedRepos  map[string]bool
	Svc           *scheduler.Service
}

// ServeHTTP handles health, run submission, status, cancel, and GitHub webhooks.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
		return
	}
	if r.URL.Path == "/v1/webhooks/github" && r.Method == http.MethodPost {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if err := security.GitHubSignature(s.WebhookSecret, r.Header.Get("X-Hub-Signature-256"), body); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var event struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if err := json.Unmarshal(body, &event); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if !s.AllowedRepos[event.Repository.FullName] {
			http.Error(w, "repository is not allowed to enqueue benchmarks", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}` + "\n"))
		return
	}
	if err := security.Bearer(r.Header.Get("Authorization"), s.Token); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/v1/runs")
	if r.URL.Path == "/v1/runs" && r.Method == http.MethodPost {
		var req struct {
			Repository     string `json:"repository"`
			ManifestPath   string `json:"manifestPath"`
			Workspace      string `json:"workspace"`
			IdempotencyKey string `json:"idempotencyKey"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = r.Header.Get("Idempotency-Key")
		}
		run, err := s.Svc.Submit(r.Context(), req.Repository, req.ManifestPath, req.Workspace, req.IdempotencyKey)
		if err != nil {
			code := http.StatusBadRequest
			if errors.Is(err, scheduler.ErrCapacity) {
				code = http.StatusTooManyRequests
			}
			http.Error(w, err.Error(), code)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(run)
		return
	}
	id := strings.Trim(path, "/")
	if id == "" || strings.Contains(id, "/") && !strings.HasSuffix(id, "/cancel") {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/cancel") && r.Method == http.MethodPost {
		id = strings.TrimSuffix(id, "/cancel")
		run, err := s.Svc.Cancel(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(run)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	run, err := s.Svc.Store.Get(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(run)
	// What: routes health, webhook, create, get, and cancel requests.
	// Used by: Listen, and internal/api/server_test.go.
	// Logic: /healthz writes ok with no auth. The webhook reads body, checks the signature, and decodes event. A repository missing from AllowedRepos gets 403 and is not queued. Other routes require Bearer. POST /v1/runs decodes req and uses the Idempotency-Key header when the JSON key is empty. Submit errors are 400, or 429 when the error is ErrCapacity. path is the URL after /v1/runs. A path ending in /cancel calls Cancel. GET loads the run by id. Any other method is 405.
}

// Listen starts the API. addr defaults to :8080 when empty.
func Listen(addr string, srv *Server) error {
	if addr == "" {
		addr = ":8080"
	}
	httpSrv := &http.Server{Addr: addr, Handler: srv, ReadHeaderTimeout: 5 * time.Second}
	return httpSrv.ListenAndServe()
	// What: blocks serving the API on addr.
	// Used by: cmd/api/main.go.
	// Logic: an empty addr becomes :8080. httpSrv uses srv as the handler and a 5 second header timeout. ListenAndServe returns only when the server stops.
}
