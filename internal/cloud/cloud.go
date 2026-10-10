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
	// What: writes one artifact file under dir.
	// Used by: internal/cloud/cloud_test.go TestArtifactRoundTrip. Get reads the same path.
	// Logic: name must be non-empty and equal to filepath.Base(name), which rejects slashes. MkdirAll creates dir. WriteFile stores body at dir/name with mode 0644.
}

// Get reads an artifact written by Put.
func Get(dir, name string) ([]byte, error) {
	if name == "" || name != filepath.Base(name) {
		return nil, fmt.Errorf("artifact name must be a single path element")
	}
	return os.ReadFile(filepath.Join(dir, name))
	// What: reads an artifact written by Put.
	// Used by: internal/cloud/cloud_test.go TestArtifactRoundTrip.
	// Logic: the same Base(name) check as Put runs first. The bytes returned are the file dir/name.
}

// EstimateUSD is cpuHours * cpuPrice + memoryGiBHours * memoryPrice.
func EstimateUSD(cpuHours, memoryGiBHours, cpuPricePerHour, memoryPricePerGiBHour float64) float64 {
	return cpuHours*cpuPricePerHour + memoryGiBHours*memoryPricePerGiBHour
	// What: adds CPU cost and memory cost.
	// Used by: internal/cloud/cloud_test.go TestBudgetAndExpiry, then passed to Allow.
	// Logic: no local variables. The first product is CPU hours times the CPU price. The second is GiB-hours times the memory price.
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
	// What: rejects a non-positive budget or an estimate above that budget.
	// Used by: internal/cloud/cloud_test.go TestBudgetAndExpiry. The API does not call it yet. benchforge_resource_budget_exceeded_total is the matching metric name.
	// Logic: budgetUSD <= 0 is an error before the comparison. estimatedUSD is only compared with budgetUSD. nil means the estimate fits.
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
	// What: lists managed resources old enough to delete. It does not delete them.
	// Used by: internal/cloud/cloud_test.go TestBudgetAndExpiry.
	// Logic: out starts empty. item is each Resource. Managed false or an empty RunID skips the item. now.Sub(item.Created) is the age. Age >= ttl appends item to out.
}
