package report

import "testing"

func TestExitCode(t *testing.T) {
	if ExitCode(&Report{Outcome: OutcomePass}) != 0 {
		t.Fatal("pass")
	}
	if ExitCode(&Report{Outcome: OutcomeRegression}) != 2 {
		t.Fatal("regression")
	}
	if ExitCode(&Report{Outcome: OutcomeInconclusive}) != 3 {
		t.Fatal("inconclusive")
	}
	if ExitCode(&Report{Outcome: OutcomeError}) != 1 {
		t.Fatal("error")
	}
	if ExitCode(&Report{Outcome: OutcomePass, CleanupError: "docker rm failed"}) != 1 {
		t.Fatal("cleanup should fail a pass")
	}
	if ExitCode(&Report{Outcome: OutcomeRegression, CleanupError: "docker rm failed"}) != 2 {
		t.Fatal("regression stays visible when cleanup also fails")
	}
}

func TestSummaryContainsOutcome(t *testing.T) {
	text := Summary(&Report{RunID: "abc", Outcome: OutcomePass, ManifestName: "orders", Workload: Workload{Type: "query"}})
	if text == "" || !contains(text, "PASS") || !contains(text, "abc") {
		t.Fatalf("summary = %s", text)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
