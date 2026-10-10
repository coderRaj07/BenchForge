package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/coderRaj07/benchforge/internal/sqlsafe"
	"github.com/jackc/pgx/v5/pgxpool"
)

func sqlFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no SQL migrations in %s", dir)
	}
	return files, nil
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	files, err := sqlFiles(dir)
	if err != nil {
		return err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		if err := sqlsafe.Check(string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", filepath.Base(path), err)
		}
		if err := conn.Conn().PgConn().Exec(ctx, string(body)).Close(); err != nil {
			return fmt.Errorf("apply %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
