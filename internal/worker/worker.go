// Package worker claims a queued run, executes it, and records the outcome.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/coderRaj07/benchforge/internal/engine"
	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/coderRaj07/benchforge/internal/scheduler"
)

// Executor runs one manifest. Tests replace it. The default calls engine.Run.
type Executor func(ctx context.Context, run scheduler.Run) (*report.Report, error)

// Cleaner removes resources labeled with the run id.
type Cleaner func(ctx context.Context, runID string) error

// Worker walks the state machine around a single execution.
type Worker struct {
	Store   *scheduler.Memory
	Execute Executor
	Clean   Cleaner
	Now     func() time.Time
}

// Process handles one job. The state walk and the engine call live in this function.
func (w *Worker) Process(ctx context.Context, job scheduler.Job) error {
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now()
	}
	run, err := w.Store.Get(ctx, job.RunID)
	if err != nil {
		return err
	}
	if run.State == scheduler.StateFailed || run.State == scheduler.StateCleanedUp {
		return nil
	}
	steps := []string{scheduler.StateValidating, scheduler.StateProvisioning, scheduler.StatePreparing, scheduler.StateWarmingUp, scheduler.StateRunning}
	for _, to := range steps {
		if err := w.Store.Transition(ctx, run.ID, run.State, to, "", now); err != nil {
			return err
		}
		run.State = to
		current, err := w.Store.Get(ctx, run.ID)
		if err != nil {
			return err
		}
		if current.State == scheduler.StateFailed {
			return w.finish(ctx, current, now)
		}
	}
	exec := w.Execute
	if exec == nil {
		exec = func(ctx context.Context, run scheduler.Run) (*report.Report, error) {
			return engine.Run(ctx, engine.Options{ManifestPath: run.ManifestPath, Workspace: run.Workspace, RunID: run.ID})
		}
	}
	runCtx, cancel := context.WithDeadline(ctx, run.Deadline)
	defer cancel()
	rep, execErr := exec(runCtx, *run)
	if err := w.Store.Transition(ctx, run.ID, scheduler.StateRunning, scheduler.StateAnalyzing, "", now); err != nil {
		return err
	}
	outcome := scheduler.StateFailed
	reason := ""
	if execErr != nil {
		reason = execErr.Error()
	} else if rep != nil {
		body, err := json.Marshal(rep)
		if err != nil {
			return err
		}
		if err := w.Store.SaveReport(ctx, run.ID, body); err != nil {
			return err
		}
		switch rep.Outcome {
		case report.OutcomePass:
			outcome = scheduler.StateCompleted
		case report.OutcomeInconclusive:
			outcome = scheduler.StateInconclusive
		case report.OutcomeRegression:
			outcome = scheduler.StateCompleted
			reason = "regression"
		default:
			outcome = scheduler.StateFailed
			reason = rep.FailureReason
		}
	}
	if err := w.Store.Transition(ctx, run.ID, scheduler.StateAnalyzing, outcome, reason, now); err != nil {
		return err
	}
	finished, err := w.Store.Get(ctx, run.ID)
	if err != nil {
		return err
	}
	return w.finish(ctx, finished, now)
	// What: moves one queued run through the state machine, executes it, and cleans up.
	// Used by: cmd/worker/main.go and internal/worker/worker_test.go.
	// Logic: now is the clock, overridable by w.Now. run is loaded by job.RunID. A run already failed or cleaned up returns immediately. steps walks validating through running, and a cancel that sets failed during that walk calls finish. exec is w.Execute or engine.Run. runCtx ends at run.Deadline. rep and execErr are the measurement. outcome becomes completed for PASS and REGRESSION, inconclusive for INCONCLUSIVE, and failed otherwise. reason stores the error text, the word regression, or the report failure. The report JSON is saved before the final transition. finished is the run after that transition, passed to finish.
}

func (w *Worker) finish(ctx context.Context, run *scheduler.Run, now time.Time) error {
	if w.Clean != nil {
		if err := w.Clean(ctx, run.ID); err != nil {
			return fmt.Errorf("cleanup %s: %w", run.ID, err)
		}
	}
	if run.State == scheduler.StateCleanedUp {
		return nil
	}
	return w.Store.Transition(ctx, run.ID, run.State, scheduler.StateCleanedUp, "", now)
	// What: runs the cleaner and records cleaned_up.
	// Used by: Process, including the path that stops early because the run was cancelled.
	// Logic: w.Clean is optional. A cleaner error is returned and the state stays where it was. A run already in cleaned_up returns nil. Otherwise Transition moves run.State to cleaned_up at now.
}
