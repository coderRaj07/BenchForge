package redact

import (
	"strings"
	"testing"
)

func TestDSN(t *testing.T) {
	in := "postgres://benchforge:s3cret@127.0.0.1:5432/benchforge?sslmode=disable"
	got := DSN(in)
	if strings.Contains(got, "s3cret") {
		t.Fatalf("password leaked: %s", got)
	}
	if !strings.Contains(got, "benchforge") {
		t.Fatalf("user was removed: %s", got)
	}

	kv := "host=127.0.0.1 password=s3cret dbname=benchforge"
	got = DSN(kv)
	if strings.Contains(got, "s3cret") {
		t.Fatalf("keyword password leaked: %s", got)
	}
}

func TestSecrets(t *testing.T) {
	got := Secrets("docker failed: PGPASSWORD=s3cret pgbench")
	if strings.Contains(got, "s3cret") {
		t.Fatalf("env secret leaked: %s", got)
	}
}
