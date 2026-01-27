package service_test

import (
	"context"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"
)

func TestStopUnknownAlias(t *testing.T) {
	s := service.NewWithClient(&config.Config{Aliases: map[string]config.Alias{}}, nil)
	if _, err := s.Stop(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error for unknown alias")
	}
}
