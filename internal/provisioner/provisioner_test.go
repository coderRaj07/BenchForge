package provisioner

import "testing"

func TestMajorVersion(t *testing.T) {
	got, err := MajorVersion("17.6 (Debian 17.6-2.pgdg12+1)")
	if err != nil || got != "17" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := MajorVersion(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestSharedBuffers(t *testing.T) {
	if got := SharedBuffersMB(1 << 30); got != 128 {
		t.Fatalf("1Gi -> %dMB, want 128", got)
	}
	if got := SharedBuffersMB(4 << 30); got != 512 {
		t.Fatalf("4Gi -> %dMB, want 512", got)
	}
}

func TestParsePublishedPort(t *testing.T) {
	got, err := parsePublishedPort("127.0.0.1:54321\n")
	if err != nil || got != "54321" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := parsePublishedPort("nope"); err == nil {
		t.Fatal("expected error")
	}
}
