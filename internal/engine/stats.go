package engine

import (
	"context"
	"fmt"

	"github.com/coderRaj07/benchforge/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbSnap struct {
	Version        string
	MaxConnections int
	SharedBuffers  string
	XactCommit     int64
	XactRollback   int64
	BlksHit        int64
	BlksRead       int64
	TupReturned    int64
	Deadlocks      int64
	Active         int
}

func snapshot(ctx context.Context, pool *pgxpool.Pool) (dbSnap, error) {
	var s dbSnap
	err := pool.QueryRow(ctx, `SELECT current_setting('server_version'), current_setting('max_connections')::int, current_setting('shared_buffers')`).Scan(&s.Version, &s.MaxConnections, &s.SharedBuffers)
	if err != nil {
		return s, fmt.Errorf("read server settings: %w", err)
	}
	err = pool.QueryRow(ctx, `
		SELECT xact_commit, xact_rollback, blks_hit, blks_read, tup_returned, deadlocks
		FROM pg_stat_database WHERE datname = current_database()`).Scan(
		&s.XactCommit, &s.XactRollback, &s.BlksHit, &s.BlksRead, &s.TupReturned, &s.Deadlocks)
	if err != nil {
		return s, fmt.Errorf("read database stats: %w", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND state = 'active'`).Scan(&s.Active); err != nil {
		return s, err
	}
	return s, nil
}

func delta(before, after dbSnap, maxActive int) report.DBStats {
	hit := after.BlksHit - before.BlksHit
	read := after.BlksRead - before.BlksRead
	ratio := 1.0
	if hit+read > 0 {
		ratio = float64(hit) / float64(hit+read)
	}
	if maxActive < after.Active {
		maxActive = after.Active
	}
	return report.DBStats{
		MaxActiveSessions: maxActive,
		XactCommit:        after.XactCommit - before.XactCommit,
		XactRollback:      after.XactRollback - before.XactRollback,
		BlksHit:           hit,
		BlksRead:          read,
		CacheHitRatio:     ratio,
		Deadlocks:         after.Deadlocks - before.Deadlocks,
		TupReturned:       after.TupReturned - before.TupReturned,
	}
}
