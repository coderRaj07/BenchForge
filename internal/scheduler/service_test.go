package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSubmitIsIdempotentAndLimited(t *testing.T) {
	ctx := context.Background()
	mem := NewMemory()
	svc := &Service{Store: mem, MaxGlobal: 2, MaxPerRepo: 1, Timeout: time.Minute}
	first, err := svc.Submit(ctx, "acme/orders", "workloads/manifests/orders-select.yaml", "/repo", "key-1")
	if err != nil {
		t.Fatal(err)
	}
	again, err := svc.Submit(ctx, "acme/orders", "workloads/manifests/orders-select.yaml", "/repo", "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID || mem.Depth() != 1 {
		t.Fatalf("duplicate submit created another job: %+v depth %d", again, mem.Depth())
	}
	if _, err := svc.Submit(ctx, "acme/orders", "workloads/manifests/orders-api.yaml", "/repo", "key-2"); !errors.Is(err, ErrCapacity) {
		t.Fatalf("expected per-repo capacity error, got %v", err)
	}
	if _, err := svc.Submit(ctx, "acme/billing", "workloads/manifests/orders-api.yaml", "/repo", "key-3"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Submit(ctx, "acme/search", "workloads/manifests/orders-api.yaml", "/repo", "key-4"); !errors.Is(err, ErrCapacity) {
		t.Fatalf("expected global capacity error, got %v", err)
	}
}

func TestCancelAndRecover(t *testing.T) {
	ctx := context.Background()
	mem := NewMemory()
	start := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	svc := &Service{Store: mem, MaxGlobal: 5, MaxPerRepo: 5, Timeout: time.Hour, Now: func() time.Time { return start }}
	run, err := svc.Submit(ctx, "acme/orders", "m.yaml", "/repo", "k")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := svc.Cancel(ctx, run.ID)
	if err != nil || cancelled.State != StateFailed || cancelled.FailureReason != "cancelled" {
		t.Fatalf("cancel: %+v %v", cancelled, err)
	}
	svc.Now = func() time.Time { return start.Add(2 * time.Hour) }
	other, err := svc.Submit(ctx, "acme/other", "m.yaml", "/repo", "k2")
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Transition(ctx, other.ID, StateQueued, StateValidating, "", start); err != nil {
		t.Fatal(err)
	}
	n, err := svc.Recover(ctx, time.Hour)
	if err != nil || n != 1 {
		t.Fatalf("recover n=%d err=%v", n, err)
	}
	got, err := mem.Get(ctx, other.ID)
	if err != nil || got.State != StateFailed || got.FailureReason != "abandoned" {
		t.Fatalf("stale run: %+v %v", got, err)
	}
}
