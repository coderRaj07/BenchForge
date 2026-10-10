// Command api serves the benchmark admission API.
package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/coderRaj07/benchforge/internal/api"
	"github.com/coderRaj07/benchforge/internal/scheduler"
)

func main() {
	token := os.Getenv("BENCHFORGE_API_TOKEN")
	if token == "" {
		log.Fatal("BENCHFORGE_API_TOKEN is required")
	}
	allowed := map[string]bool{}
	for _, name := range strings.Split(os.Getenv("BENCHFORGE_ALLOWED_REPOS"), ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			allowed[name] = true
		}
	}
	if err := api.Listen(os.Getenv("BENCHFORGE_API_ADDR"), &api.Server{
		Token:         token,
		WebhookSecret: os.Getenv("BENCHFORGE_WEBHOOK_SECRET"),
		AllowedRepos:  allowed,
		Svc: &scheduler.Service{
			Store:      scheduler.NewMemory(),
			MaxGlobal:  4,
			MaxPerRepo: 2,
			Timeout:    30 * time.Minute,
		},
	}); err != nil {
		log.Fatal(err)
	}
	// What: starts the HTTP API and blocks until it stops.
	// Used by: the benchforge-api process, package main only.
	// Logic: token is BENCHFORGE_API_TOKEN and an empty value stops the process. allowed starts empty and gains each comma-separated name from BENCHFORGE_ALLOWED_REPOS. The Server uses that token, BENCHFORGE_WEBHOOK_SECRET, and a new in-memory scheduler limited to 4 global runs and 2 per repository with a 30 minute timeout. Listen uses BENCHFORGE_API_ADDR.
}
