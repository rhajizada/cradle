package service

import (
	"context"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
)

func TestStopUnknownAlias(t *testing.T) {
	s := &Service{cfg: &config.Config{Aliases: map[string]config.Alias{}}}
	if _, err := s.Stop(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error for unknown alias")
	}
}
