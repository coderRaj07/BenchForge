package runner

import (
	"strings"
	"testing"

	"github.com/coderRaj07/benchforge/internal/manifest"
	"github.com/coderRaj07/benchforge/internal/seed"
)

const sampleStdout = `
transaction type: multiple scripts
scaling factor: 1
query mode: simple
number of clients: 4
number of threads: 2
duration: 5 s
number of transactions actually processed: 1000
number of failed transactions: 2 (0.200%)
number of transactions skipped: 3 (0.299%)
latency average = 2.500 ms
latency stddev = 0.400 ms
initial connection time = 10.000 ms
tps = 200.123456 (without initial connection time)
`

func TestParsePgbench(t *testing.T) {
	got := parsePgbench(sampleStdout)
	if got.Transactions != 1000 || got.Failed != 2 || got.Skipped != 3 {
		t.Fatalf("counts %+v", got)
	}
	if got.TPS < 200 || got.TPS > 201 {
		t.Fatalf("tps %v", got.TPS)
	}
	if got.Latency.Mean != 2.5 || got.StddevMs != 0.4 {
		t.Fatalf("latency %+v stddev %v", got.Latency, got.StddevMs)
	}
}

func TestPgbenchScriptSubstitutesPlaceholders(t *testing.T) {
	q := manifest.Query{
		Params: []manifest.Param{{
			Name: "customer_id", Type: "int", Distribution: "uniform", Min: 1, MaxFrom: "customers",
		}},
		Statements: []string{"SELECT id FROM orders WHERE customer_id = $1"},
		Weight:     1,
	}
	body, err := pgbenchScript(q, seed.Counts{Customers: 25})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `\set customer_id random(1, 25)`) {
		t.Fatalf("missing set: %s", body)
	}
	if !strings.Contains(body, "customer_id = :customer_id") {
		t.Fatalf("placeholder not replaced: %s", body)
	}
	if strings.Contains(body, "$1") {
		t.Fatalf("raw placeholder remained: %s", body)
	}
}
