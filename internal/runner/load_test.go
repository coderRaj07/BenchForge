package runner

import "testing"

func TestClassifyLoadBackpressureIsNotSaturation(t *testing.T) {
	sat, reason := ClassifyLoad(1000, 400, 0, 1000)
	if sat || reason != "" {
		t.Fatalf("backpressure was treated as saturation: %v %s", sat, reason)
	}
}

func TestClassifyLoadLag(t *testing.T) {
	sat, reason := ClassifyLoad(1000, 500, 300, 1000)
	if !sat || reason == "" {
		t.Fatalf("expected saturation, got %v %q", sat, reason)
	}
}

func TestClassifyLoadKeepsUp(t *testing.T) {
	sat, _ := ClassifyLoad(1000, 990, 10, 1000)
	if sat {
		t.Fatal("a generator that kept up should not be saturated")
	}
}
