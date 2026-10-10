-- Metadata for benchmark runs. Applied by the API process to its own
-- database, never by a pull-request manifest.

CREATE TABLE runs (
    id uuid PRIMARY KEY,
    idempotency_key text NOT NULL UNIQUE,
    repository text NOT NULL,
    state text NOT NULL,
    failure_reason text NOT NULL DEFAULT '',
    manifest_path text NOT NULL,
    workspace text NOT NULL DEFAULT '',
    report_json jsonb,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    deadline_at timestamptz NOT NULL,
    CONSTRAINT runs_state_check CHECK (state IN (
        'queued', 'validating', 'provisioning', 'preparing', 'warming_up',
        'running', 'analyzing', 'completed', 'failed', 'inconclusive', 'cleaned_up'
    ))
);

CREATE INDEX runs_state_updated_idx ON runs (state, updated_at);
CREATE INDEX runs_repository_state_idx ON runs (repository, state);

CREATE TABLE run_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id uuid NOT NULL REFERENCES runs (id),
    state text NOT NULL,
    message text NOT NULL DEFAULT '',
    at timestamptz NOT NULL
);

CREATE INDEX run_events_run_idx ON run_events (run_id, id);
