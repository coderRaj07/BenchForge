package scheduler

import "testing"

func TestTransitions(t *testing.T) {
	if !CanTransition(StateQueued, StateValidating) {
		t.Fatal("queued should enter validating")
	}
	if CanTransition(StateQueued, StateRunning) {
		t.Fatal("queued must not jump to running")
	}
	if CanTransition(StateCleanedUp, StateFailed) {
		t.Fatal("cleaned_up is terminal")
	}
	if CanTransition(StateAnalyzing, StateInconclusive) != true {
		t.Fatal("analyzing can be inconclusive")
	}
	if !CanTransition(StateFailed, StateCleanedUp) {
		t.Fatal("failed runs must be cleanable")
	}
}

func TestActiveSlot(t *testing.T) {
	if !Active(StateRunning) || !Active(StateCompleted) {
		t.Fatal("completed still holds a slot until cleanup")
	}
	if Active(StateCleanedUp) {
		t.Fatal("cleaned_up must not hold a slot")
	}
}
