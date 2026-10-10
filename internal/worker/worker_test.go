package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/coderRaj07/benchforge/internal/scheduler"
)

func TestProcessRecordsOutcomeAndCleansUp(t *testing.T) {
	ctx := context.Background()
	mem := scheduler.NewMemory()
	svc := &scheduler.Service{Store: mem, MaxGlobal: 2, MaxPerRepo: 2, Timeout: time.Minute}
	run, err := svc.Submit(ctx, "acme/orders", "workloads/manifests/orders-select.yaml", "/repo", "k")
	if err != nil {
		t.Fatal(err)
	}
	job, ok, err := mem.Receive(ctx)
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	cleaned := ""
	w := &Worker{
		Store: mem,
		Execute: func(context.Context, scheduler.Run) (*report.Report, error) {
			return &report.Report{Outcome: report.OutcomePass, RunID: run.ID}, nil
		},
		Clean: func(_ context.Context, id string) error {
			cleaned = id
			return nil
		},
	}
	if err := w.Process(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, err := mem.Get(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != scheduler.StateCleanedUp || cleaned != run.ID || len(got.ReportJSON) == 0 {
		t.Fatalf("state %s cleaned %s report %d", got.State, cleaned, len(got.ReportJSON))
	}
}

func TestProcessFailureStillCleansUp(t *testing.T) {
	ctx := context.Background()
	mem := scheduler.NewMemory()
	svc := &scheduler.Service{Store: mem, MaxGlobal: 1, MaxPerRepo: 1, Timeout: time.Minute}
	run, err := svc.Submit(ctx, "acme/orders", "m.yaml", "/repo", "k")
	if err != nil {
		t.Fatal(err)
	}
	job, _, _ := mem.Receive(ctx)
	cleaned := false
	w := &Worker{
		Store: mem,
		Execute: func(context.Context, scheduler.Run) (*report.Report, error) {
			return nil, errors.New("docker failed")
		},
		Clean: func(context.Context, string) error {
			cleaned = true
			return nil
		},
	}
	if err := w.Process(ctx, job); err != nil {
		t.Fatal(err)
	}
	got, _ := mem.Get(ctx, run.ID)
	if got.State != scheduler.StateCleanedUp || got.FailureReason != "docker failed" || !cleaned {
		t.Fatalf("%+v cleaned %v", got, cleaned)
	}
}
