package canary

import "testing"

func TestDecide(t *testing.T) {
	steps := []int{10, 25, 50, 100}
	if action, _ := Decide(10, 10, 0, 3, 30, 10, 0.01, steps, 10); action != "pause" {
		t.Fatal(action)
	}
	if action, _ := Decide(10, 10, 0.2, 100, 30, 10, 0.01, steps, 10); action != "rollback" {
		t.Fatal(action)
	}
	if action, reason := Decide(10.5, 10, 0, 100, 30, 10, 0.01, steps, 10); action != "promote" || reason == "" {
		t.Fatalf("%s %s", action, reason)
	}
	if action, _ := Decide(30, 10, 0, 100, 30, 10, 0.01, steps, 25); action != "rollback" {
		t.Fatal(action)
	}
	if action, reason := Decide(10, 10, 0, 100, 30, 10, 0.01, steps, 100); action != "promote" {
		t.Fatal(action, reason)
	}
}
