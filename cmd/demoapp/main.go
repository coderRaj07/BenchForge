// Command demoapp is a local HTTP target for a k6 script.
// It is not a production service and it does not open a database.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

func main() {
	addr := os.Getenv("BENCHFORGE_DEMO_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8081"
	}
	srv := &http.Server{Addr: addr, Handler: newHandler(4), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("demo app listening on %s", addr)
	log.Fatal(srv.ListenAndServe())
	// What: serves the local HTTP target used by a k6 script.
	// Used by: the demoapp process, package main only. cmd/demoapp/main_test.go calls newHandler, not main.
	// Logic: addr is BENCHFORGE_DEMO_ADDR, or 127.0.0.1:8081 when that variable is empty. srv uses newHandler(4), so four requests can hold a slot at once.
}

func newHandler(limit int) http.Handler {
	slots := make(chan struct{}, limit)
	var mu sync.Mutex
	var waits []time.Duration
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /orders", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		select {
		case slots <- struct{}{}:
			mu.Lock()
			waits = append(waits, time.Since(start))
			mu.Unlock()
			defer func() { <-slots }()
		case <-r.Context().Done():
			http.Error(w, "cancelled", http.StatusRequestTimeout)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"customerId": r.URL.Query().Get("customer_id"), "orders": []any{}})
	})
	mux.HandleFunc("GET /pool-waits", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"samples": len(waits)})
	})
	return mux
	// What: builds the demo HTTP routes and a fixed-size slot pool.
	// Used by: main, and cmd/demoapp/main_test.go TestOrdersAndHealth.
	// Logic: slots is a buffered channel of size limit. waits records how long each /orders request waited for a slot, guarded by mu. /healthz writes ok. /orders records time.Since(start) after a slot is taken and releases the slot on return. A cancelled request gets status 408. /pool-waits returns the number of recorded waits.
}
