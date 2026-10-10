// Command worker drains queued runs from a store the API process shares.
// This binary uses the in-memory store only when BENCHFORGE_WORKER_ONCE is set
// for a local demonstration. Production sharing of the queue is the Postgres
// schema in migrations/001_runs.sql, applied to the API database.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coderRaj07/benchforge/internal/scheduler"
	"github.com/coderRaj07/benchforge/internal/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store := scheduler.NewMemory()
	w := &worker.Worker{Store: store}
	log.Printf("worker waiting; set a shared store before relying on this process across restarts")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, ok, err := store.Receive(ctx)
			if err != nil {
				log.Fatal(err)
			}
			if !ok {
				if os.Getenv("BENCHFORGE_WORKER_ONCE") == "1" {
					os.Exit(0)
				}
				continue
			}
			if err := w.Process(ctx, job); err != nil {
				log.Printf("run %s: %v", job.RunID, err)
			}
		}
	}
	// What: polls an in-memory queue and runs each job until the process is signalled.
	// Used by: the benchforge-worker process, package main only.
	// Logic: ctx ends on SIGINT or SIGTERM. store and w are a new memory queue and worker that do not share state with another process. ticker fires once a second. job, ok, err come from Receive. An empty queue exits when BENCHFORGE_WORKER_ONCE is 1, otherwise the loop waits. Process errors are logged and the loop continues.
}
