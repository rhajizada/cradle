package service

import "testing"

func TestParsePlatform(t *testing.T) {
	p, err := parsePlatform("linux/amd64")
	if err != nil {
		t.Fatalf("parsePlatform error: %v", err)
	}
	if p.OS != "linux" || p.Architecture != "amd64" {
		t.Fatalf("unexpected platform: %+v", p)
	}
}

func TestParsePlatformInvalid(t *testing.T) {
	if _, err := parsePlatform("bad"); err == nil {
		t.Fatalf("expected error for bad platform")
	}
}
