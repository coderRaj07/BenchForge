// Package cloud estimates cost, stores artifacts on the local disk, and
// chooses which labeled resources are old enough to delete.
package cloud

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Put writes one artifact under dir. The name may not contain a path separator.
func Put(dir, name string, body []byte) error {
	if name == "" || name != filepath.Base(name) {
		return fmt.Errorf("artifact name must be a single path element")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), body, 0o644)
}

// Get reads an artifact written by Put.
func Get(dir, name string) ([]byte, error) {
	if name == "" || name != filepath.Base(name) {
		return nil, fmt.Errorf("artifact name must be a single path element")
	}
	return os.ReadFile(filepath.Join(dir, name))
}

// EstimateUSD is cpuHours * cpuPrice + memoryGiBHours * memoryPrice.
func EstimateUSD(cpuHours, memoryGiBHours, cpuPricePerHour, memoryPricePerGiBHour float64) float64 {
	return cpuHours*cpuPricePerHour + memoryGiBHours*memoryPricePerGiBHour
}

// Allow returns an error when estimated cost is above the budget.
func Allow(estimatedUSD, budgetUSD float64) error {
	if budgetUSD <= 0 {
		return fmt.Errorf("budget must be greater than zero")
	}
	if estimatedUSD > budgetUSD {
		return fmt.Errorf("estimated $%.4f exceeds budget $%.4f", estimatedUSD, budgetUSD)
	}
	return nil
}

// Resource is one temporary object the reconciler may delete.
type Resource struct {
	ID      string
	Kind    string
	RunID   string
	Created time.Time
	Managed bool
}

// Expired returns managed resources whose run id is set and whose age is past ttl.
// Unmanaged resources and resources with an empty run id are never selected.
func Expired(now time.Time, ttl time.Duration, items []Resource) []Resource {
	var out []Resource
	for _, item := range items {
		if !item.Managed || item.RunID == "" {
			continue
		}
		if now.Sub(item.Created) >= ttl {
			out = append(out, item)
		}
	}
	return out
}
