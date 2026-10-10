package seed

import "testing"

func TestBuildDeterministic(t *testing.T) {
	a, err := Build(42, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(42, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Checksum() != b.Checksum() {
		t.Fatal("same seed produced different data")
	}
	c, err := Build(43, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if a.Checksum() == c.Checksum() {
		t.Fatal("different seed produced the same checksum")
	}
}

func TestSizeScaleOne(t *testing.T) {
	c, p, o := Size(1)
	if c != 1000 || p != 200 || o != 10000 {
		t.Fatalf("got %d %d %d", c, p, o)
	}
}

func TestInvariants(t *testing.T) {
	d, err := Build(7, 0.05)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Invariants(); err != nil {
		t.Fatal(err)
	}
	counts := d.Counts()
	if counts.Orders != int64(len(d.Orders)) || counts.OrderItems != int64(len(d.Items)) {
		t.Fatalf("counts %+v", counts)
	}
	max, err := counts.Max("customers")
	if err != nil || max != counts.Customers {
		t.Fatalf("max customers %d err %v", max, err)
	}
}
