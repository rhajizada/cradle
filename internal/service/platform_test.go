package service_test

import (
	"testing"

	"github.com/rhajizada/cradle/internal/service"
)

func TestParsePlatform(t *testing.T) {
	p, err := service.ParsePlatform("linux/amd64")
	if err != nil {
		t.Fatalf("parsePlatform error: %v", err)
	}
	if p.OS != "linux" || p.Architecture != "amd64" {
		t.Fatalf("unexpected platform: %+v", p)
	}
}

func TestParsePlatformInvalid(t *testing.T) {
	if _, err := service.ParsePlatform("bad"); err == nil {
		t.Fatalf("expected error for bad platform")
	}
}
