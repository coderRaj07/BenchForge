package provisioner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coderRaj07/benchforge/internal/manifest"
	"github.com/coderRaj07/benchforge/internal/redact"
)

// Docker provisions a disposable PostgreSQL container.
type Docker struct {
	// Bin is the docker binary. Empty means "docker" on PATH.
	Bin string
}

// NewPassword returns a random password for an ephemeral role.
func NewPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// Provision starts PostgreSQL and waits until it accepts connections.
func (d *Docker) Provision(ctx context.Context, req Request) (*Environment, error) {
	bin := d.Bin
	if bin == "" {
		bin = "docker"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return nil, fmt.Errorf("docker was not found on PATH; install Docker or pass an external DSN with --allow-external")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("ephemeral postgres requires a generated password")
	}
	name := containerName(req.RunID)
	sb := SharedBuffersMB(req.MemoryBytes)
	maxConn := req.MaxConnections
	if maxConn < 40 {
		maxConn = 40
	}
	envFile, err := writeEnvFile(req.Password)
	if err != nil {
		return nil, err
	}
	defer os.Remove(envFile)

	args := []string{
		"run", "-d",
		"--name", name,
		"--label", "benchforge.managed=true",
		"--label", "benchforge.run_id=" + req.RunID,
		"--memory", manifest.FormatMemory(req.MemoryBytes),
		"--cpus", fmt.Sprintf("%.2f", req.CPU),
		"--env-file", envFile,
		"-p", "127.0.0.1::5432",
		req.Image,
		"postgres",
		"-c", fmt.Sprintf("max_connections=%d", maxConn),
		"-c", fmt.Sprintf("shared_buffers=%dMB", sb),
	}
	if _, err := d.output(ctx, args...); err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}
	env := &Environment{
		RunID:         req.RunID,
		ContainerName: name,
		Image:         req.Image,
		User:          User,
		Password:      req.Password,
		Database:      Database,
		Host:          "127.0.0.1",
		Port:          "5432",
		InContainer:   true,
		logPrefix:     "/tmp/bfpg",
	}
	env.remove = func(ctx context.Context) error {
		cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := d.output(cctx, "rm", "-f", name)
		return err
	}
	env.stage = func(ctx context.Context, local string) (string, error) {
		remote := "/tmp/" + filepath.Base(local)
		if _, err := d.output(ctx, "cp", local, name+":"+remote); err != nil {
			return "", fmt.Errorf("copy pgbench script: %w", err)
		}
		return remote, nil
	}
	env.run = func(ctx context.Context, benchArgs []string) (string, error) {
		full := []string{"pgbench", "-h", "127.0.0.1", "-p", "5432", "-U", User, "-d", Database}
		full = append(full, benchArgs...)
		return d.exec(ctx, name, []string{"PGPASSWORD=" + req.Password}, "/tmp", full...)
	}
	env.collect = func(ctx context.Context, dest string) ([]string, error) {
		listing, err := d.exec(ctx, name, nil, "/tmp", "ls", "-1", "/tmp")
		if err != nil {
			return nil, err
		}
		var files []string
		for _, line := range strings.Split(listing, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "bfpg") {
				continue
			}
			local := filepath.Join(dest, line)
			if _, err := d.output(ctx, "cp", name+":/tmp/"+line, local); err != nil {
				return files, err
			}
			files = append(files, local)
		}
		return files, nil
	}

	readyCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	if err := d.waitReady(readyCtx, name); err != nil {
		_ = env.Remove(ctx)
		return nil, err
	}
	portOut, err := d.output(ctx, "port", name, "5432/tcp")
	if err != nil {
		_ = env.Remove(ctx)
		return nil, fmt.Errorf("read published port: %w", err)
	}
	hostPort, err := parsePublishedPort(portOut)
	if err != nil {
		_ = env.Remove(ctx)
		return nil, err
	}
	env.DSN = dsn(req.Password, hostPort)
	if id, err := d.output(ctx, "inspect", "-f", "{{.Image}}", name); err == nil {
		env.ImageID = strings.TrimSpace(id)
	}
	return env, nil
}

func (d *Docker) waitReady(ctx context.Context, name string) error {
	for {
		if err := ctx.Err(); err != nil {
			logs, _ := d.output(context.Background(), "logs", "--tail", "40", name)
			return fmt.Errorf("postgres did not become ready: %w\n%s", err, trimLog(logs))
		}
		logs, _ := d.output(ctx, "logs", name)
		// The official image accepts connections once during init, then restarts.
		// Wait until that restart has been announced before trusting pg_isready.
		if strings.Contains(logs, "PostgreSQL init process complete") {
			if _, err := d.exec(ctx, name, nil, "", "pg_isready", "-U", User, "-d", Database); err == nil {
				if _, err := d.exec(ctx, name, nil, "", "psql", "-U", User, "-d", Database, "-v", "ON_ERROR_STOP=1", "-c", "SELECT 1"); err == nil {
					return nil
				}
			}
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func trimLog(s string) string {
	if len(s) > 2000 {
		return s[len(s)-2000:]
	}
	return s
}

func (d *Docker) exec(ctx context.Context, container string, env []string, workdir string, command ...string) (string, error) {
	args := []string{"exec"}
	if workdir != "" {
		args = append(args, "-w", workdir)
	}
	for _, e := range env {
		args = append(args, "-e", e)
	}
	args = append(args, container)
	args = append(args, command...)
	return d.output(ctx, args...)
}

func (d *Docker) output(ctx context.Context, args ...string) (string, error) {
	bin := d.Bin
	if bin == "" {
		bin = "docker"
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, redact.Secrets(text))
	}
	return text, nil
}

func writeEnvFile(password string) (string, error) {
	f, err := os.CreateTemp("", "benchforge-env-")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if _, err := fmt.Fprintf(f, "POSTGRES_USER=%s\nPOSTGRES_PASSWORD=%s\nPOSTGRES_DB=%s\n", User, password, Database); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func dsn(password, port string) string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(User, password),
		Host:     net.JoinHostPort("127.0.0.1", port),
		Path:     Database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func containerName(runID string) string {
	compact := strings.ReplaceAll(runID, "-", "")
	if len(compact) > 12 {
		compact = compact[:12]
	}
	if compact == "" {
		compact = "local"
	}
	return "bf-" + compact
}

func parsePublishedPort(out string) (string, error) {
	line := strings.TrimSpace(out)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	i := strings.LastIndex(line, ":")
	if i < 0 || i == len(line)-1 {
		return "", fmt.Errorf("unexpected docker port output %q", out)
	}
	return line[i+1:], nil
}

// External returns an environment that uses a caller-supplied DSN.
// It does not create or delete infrastructure.
func External(runID, dsn string) *Environment {
	return &Environment{
		RunID:    runID,
		DSN:      dsn,
		User:     User,
		Database: Database,
		Host:     "127.0.0.1",
		Port:     "5432",
	}
}
