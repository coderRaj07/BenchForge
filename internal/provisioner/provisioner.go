// Package provisioner starts and removes isolated PostgreSQL environments.
// The image is chosen by BenchForge, never by a manifest-supplied string.
package provisioner

import (
	"context"
	"fmt"
	"strings"
)

const (
	// User is the role created inside an ephemeral database.
	User = "benchforge"
	// Database is the database created inside an ephemeral instance.
	Database = "benchforge"
)

// Request asks for one disposable PostgreSQL instance.
type Request struct {
	RunID          string
	Image          string
	CPU            float64
	MemoryBytes    int64
	MaxConnections int
	Password       string
}

// Environment is a running database plus the commands needed to drive pgbench
// in that environment.
type Environment struct {
	RunID         string
	DSN           string
	ContainerName string
	Image         string
	ImageID       string
	User          string
	Password      string
	Database      string
	// Host and Port are where pgbench connects. Inside a container that is
	// 127.0.0.1:5432. For an external database it is the published address.
	Host        string
	Port        string
	InContainer bool
	remove      func(ctx context.Context) error
	stage       func(ctx context.Context, local string) (string, error)
	run         func(ctx context.Context, args []string) (string, error)
	logPrefix   string
	collect     func(ctx context.Context, dest string) ([]string, error)
}

// Remove deletes ephemeral resources. It is safe to call on an external environment.
func (e *Environment) Remove(ctx context.Context) error {
	if e == nil || e.remove == nil {
		return nil
	}
	return e.remove(ctx)
}

// Stage places a local script where pgbench will read it and returns that path.
func (e *Environment) Stage(ctx context.Context, localPath string) (string, error) {
	if e.stage == nil {
		return "", fmt.Errorf("this environment cannot stage pgbench scripts")
	}
	return e.stage(ctx, localPath)
}

// RunPgbench executes pgbench with connection flags supplied by the environment.
// args are benchmark flags only. The password is not placed in the argument list
// returned to callers; command errors are redacted.
func (e *Environment) RunPgbench(ctx context.Context, args []string) (string, error) {
	if e.run == nil {
		return "", fmt.Errorf("pgbench is not available for this environment; install the PostgreSQL client tools or use an ephemeral source")
	}
	return e.run(ctx, args)
}

// LogPrefix is the pgbench --log-prefix path in the environment that runs pgbench.
func (e *Environment) LogPrefix() string {
	if e.logPrefix == "" {
		return "pgbench_log"
	}
	return e.logPrefix
}

// CollectLogs copies pgbench transaction logs into dest and returns local paths.
func (e *Environment) CollectLogs(ctx context.Context, dest string) ([]string, error) {
	if e.collect == nil {
		return nil, nil
	}
	return e.collect(ctx, dest)
}

// MajorVersion extracts the major component from a server_version string
// such as "17.6 (Debian 17.6-2.pgdg12+1)".
func MajorVersion(serverVersion string) (string, error) {
	serverVersion = strings.TrimSpace(serverVersion)
	if serverVersion == "" {
		return "", fmt.Errorf("empty postgres version")
	}
	major := serverVersion
	if i := strings.IndexAny(major, " ."); i >= 0 {
		major = major[:i]
	}
	if major == "" || strings.ContainsAny(major, "abcdefghijklmnopqrstuvwxyz") {
		return "", fmt.Errorf("cannot parse postgres version %q", serverVersion)
	}
	return major, nil
}

// SharedBuffersMB picks shared_buffers from the container memory budget.
// It uses one eighth of the memory, clamped to 16MiB–1GiB and never more than a quarter.
func SharedBuffersMB(memoryBytes int64) int {
	mb := memoryBytes / 8 / (1024 * 1024)
	capMB := memoryBytes / 4 / (1024 * 1024)
	if capMB < 16 {
		capMB = 16
	}
	if mb > capMB {
		mb = capMB
	}
	if mb < 16 {
		mb = 16
	}
	if mb > 1024 {
		mb = 1024
	}
	return int(mb)
}
