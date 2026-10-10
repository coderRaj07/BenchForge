package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// States of a benchmark run. cleaned_up is the only state with no successor.
const (
	StateQueued       = "queued"
	StateValidating   = "validating"
	StateProvisioning = "provisioning"
	StatePreparing    = "preparing"
	StateWarmingUp    = "warming_up"
	StateRunning      = "running"
	StateAnalyzing    = "analyzing"
	StateCompleted    = "completed"
	StateFailed       = "failed"
	StateInconclusive = "inconclusive"
	StateCleanedUp    = "cleaned_up"
)

var next = map[string]map[string]bool{
	StateQueued:       {StateValidating: true, StateFailed: true},
	StateValidating:   {StateProvisioning: true, StateFailed: true},
	StateProvisioning: {StatePreparing: true, StateFailed: true},
	StatePreparing:    {StateWarmingUp: true, StateFailed: true},
	StateWarmingUp:    {StateRunning: true, StateFailed: true},
	StateRunning:      {StateAnalyzing: true, StateFailed: true},
	StateAnalyzing:    {StateCompleted: true, StateFailed: true, StateInconclusive: true},
	StateCompleted:    {StateCleanedUp: true},
	StateFailed:       {StateCleanedUp: true},
	StateInconclusive: {StateCleanedUp: true},
	StateCleanedUp:    {},
}

// ErrConflict is returned when the run is no longer in the expected state.
var ErrConflict = errors.New("run state conflict")

// ErrCapacity is returned when a global or per-repository limit is full.
var ErrCapacity = errors.New("run capacity exceeded")

// ErrNotFound is returned when the id does not exist.
var ErrNotFound = errors.New("run not found")

// CanTransition reports whether from may move directly to to.
func CanTransition(from, to string) bool {
	return next[from][to]
}

// Active reports whether the run still occupies a concurrency slot.
// cleaned_up does not. Terminal results do, until cleanup finishes.
func Active(state string) bool {
	return state != StateCleanedUp && state != ""
}

// Run is the persisted record.
type Run struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotencyKey"`
	Repository     string    `json:"repository"`
	State          string    `json:"state"`
	FailureReason  string    `json:"failureReason,omitempty"`
	ManifestPath   string    `json:"manifestPath"`
	Workspace      string    `json:"workspace,omitempty"`
	ReportJSON     []byte    `json:"-"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Deadline       time.Time `json:"deadline"`
	Events         []Event   `json:"events,omitempty"`
}

// Event is one accepted transition.
type Event struct {
	State   string    `json:"state"`
	Message string    `json:"message,omitempty"`
	At      time.Time `json:"at"`
}

// Job is one unit of queue work.
type Job struct {
	RunID   string
	Receipt string
}

// Memory is a process-local store and queue for tests and the single-process worker.
type Memory struct {
	mu    sync.Mutex
	runs  map[string]*Run
	byKey map[string]string
	queue []Job
	seq   int
}

// NewMemory returns an empty store.
func NewMemory() *Memory {
	return &Memory{runs: map[string]*Run{}, byKey: map[string]string{}}
}

// Insert stores a new run. A repeated idempotency key returns the existing row.
func (m *Memory) Insert(_ context.Context, run Run) (bool, *Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id, ok := m.byKey[run.IdempotencyKey]; ok {
		cp := clone(m.runs[id])
		return false, cp, nil
	}
	if run.State == "" {
		run.State = StateQueued
	}
	run.Events = []Event{{State: run.State, At: run.CreatedAt}}
	stored := run
	m.runs[run.ID] = &stored
	m.byKey[run.IdempotencyKey] = run.ID
	return true, clone(&stored), nil
}

// Get returns a copy of the run.
func (m *Memory) Get(_ context.Context, id string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(run), nil
}

// Transition moves from to to when CanTransition allows it.
func (m *Memory) Transition(_ context.Context, id, from, to, message string, at time.Time) error {
	if !CanTransition(from, to) {
		return ErrConflict
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return ErrNotFound
	}
	if run.State != from {
		return ErrConflict
	}
	run.State = to
	run.UpdatedAt = at
	if message != "" {
		run.FailureReason = message
	}
	run.Events = append(run.Events, Event{State: to, Message: message, At: at})
	return nil
}

// CountActive counts runs that still hold a slot.
func (m *Memory) CountActive(_ context.Context, repository string) (int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	global, repo := 0, 0
	for _, run := range m.runs {
		if !Active(run.State) {
			continue
		}
		global++
		if run.Repository == repository {
			repo++
		}
	}
	return global, repo, nil
}

// ListStale returns active runs whose update time is before cutoff.
func (m *Memory) ListStale(_ context.Context, cutoff time.Time) ([]Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Run
	for _, run := range m.runs {
		if Active(run.State) && run.UpdatedAt.Before(cutoff) {
			out = append(out, *clone(run))
		}
	}
	return out, nil
}

// SaveReport attaches the report JSON without changing state.
func (m *Memory) SaveReport(_ context.Context, id string, body []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return ErrNotFound
	}
	run.ReportJSON = append([]byte(nil), body...)
	return nil
}

// Enqueue adds a job.
func (m *Memory) Enqueue(_ context.Context, job Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	if job.Receipt == "" {
		job.Receipt = job.RunID
	}
	m.queue = append(m.queue, job)
	return nil
}

// Receive takes the next job. The second result is false when the queue is empty.
func (m *Memory) Receive(_ context.Context) (Job, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) == 0 {
		return Job{}, false, nil
	}
	job := m.queue[0]
	m.queue = m.queue[1:]
	return job, true, nil
}

// Depth is the number of queued jobs.
func (m *Memory) Depth() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

func clone(in *Run) *Run {
	if in == nil {
		return nil
	}
	cp := *in
	cp.Events = append([]Event(nil), in.Events...)
	cp.ReportJSON = append([]byte(nil), in.ReportJSON...)
	return &cp
}

// Service admits runs and recovers ones whose worker stopped updating them.
type Service struct {
	Store      *Memory
	MaxGlobal  int
	MaxPerRepo int
	Timeout    time.Duration
	Now        func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

// Submit stores a queued run and enqueues it. The same idempotency key returns the original run.
func (s *Service) Submit(ctx context.Context, repository, manifestPath, workspace, idempotencyKey string) (*Run, error) {
	if repository == "" || manifestPath == "" || idempotencyKey == "" {
		return nil, fmt.Errorf("repository, manifest path, and idempotency key are required")
	}
	if existingID, ok := s.Store.byKeyLocked(idempotencyKey); ok {
		return s.Store.Get(ctx, existingID)
	}
	global, repo, err := s.Store.CountActive(ctx, repository)
	if err != nil {
		return nil, err
	}
	if s.MaxGlobal > 0 && global >= s.MaxGlobal {
		return nil, fmt.Errorf("%w: global limit %d", ErrCapacity, s.MaxGlobal)
	}
	if s.MaxPerRepo > 0 && repo >= s.MaxPerRepo {
		return nil, fmt.Errorf("%w: repository limit %d", ErrCapacity, s.MaxPerRepo)
	}
	at := s.now()
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	run := Run{
		ID:             uuid.NewString(),
		IdempotencyKey: idempotencyKey,
		Repository:     repository,
		State:          StateQueued,
		ManifestPath:   manifestPath,
		Workspace:      workspace,
		CreatedAt:      at,
		UpdatedAt:      at,
		Deadline:       at.Add(timeout),
	}
	created, stored, err := s.Store.Insert(ctx, run)
	if err != nil {
		return nil, err
	}
	if !created {
		return stored, nil
	}
	if err := s.Store.Enqueue(ctx, Job{RunID: stored.ID, Receipt: stored.ID}); err != nil {
		return nil, err
	}
	return stored, nil
}

// Cancel moves a non-terminal run to failed. The caller still has to clean up.
func (s *Service) Cancel(ctx context.Context, id string) (*Run, error) {
	run, err := s.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.State == StateCleanedUp {
		return run, nil
	}
	if run.State == StateFailed || run.State == StateCompleted || run.State == StateInconclusive {
		return run, nil
	}
	if err := s.Store.Transition(ctx, id, run.State, StateFailed, "cancelled", s.now()); err != nil {
		return nil, err
	}
	return s.Store.Get(ctx, id)
}

// Recover marks stale active runs failed so a cleanup pass can collect them.
func (s *Service) Recover(ctx context.Context, staleFor time.Duration) (int, error) {
	if staleFor <= 0 {
		staleFor = 15 * time.Minute
	}
	stale, err := s.Store.ListStale(ctx, s.now().Add(-staleFor))
	if err != nil {
		return 0, err
	}
	n := 0
	for _, run := range stale {
		if run.State == StateFailed || run.State == StateCompleted || run.State == StateInconclusive {
			continue
		}
		if err := s.Store.Transition(ctx, run.ID, run.State, StateFailed, "abandoned", s.now()); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (m *Memory) byKeyLocked(key string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byKey[key]
	return id, ok
}
