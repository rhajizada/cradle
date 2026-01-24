package service_test

import (
	"context"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
	"github.com/rhajizada/cradle/internal/service"
)

func TestAliasInfoAndListAliases(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"b": {Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}}},
			"a": {Image: config.ImageSpec{Build: &config.BuildSpec{Cwd: "/tmp"}}},
		},
	}

	s := service.NewWithClient(cfg, nil)

	infos := s.ListAliases()
	if len(infos) != 2 || infos[0].Name != "a" || infos[1].Name != "b" {
		t.Fatalf("expected sorted aliases, got %+v", infos)
	}

	info, err := s.AliasInfo("b")
	if err != nil {
		t.Fatalf("AliasInfo error: %v", err)
	}
	if info.Kind != service.ImagePull || info.Ref != "ubuntu:24.04" {
		t.Fatalf("unexpected pull info: %+v", info)
	}
	if info.Description() != "pull: ubuntu:24.04" {
		t.Fatalf("unexpected description: %q", info.Description())
	}

	info, err = s.AliasInfo("a")
	if err != nil {
		t.Fatalf("AliasInfo error: %v", err)
	}
	if info.Kind != service.ImageBuild || info.Tag == "" {
		t.Fatalf("unexpected build info: %+v", info)
	}
	if info.Description() == "" {
		t.Fatalf("expected description for build")
	}
}

func TestAliasInfoErrors(t *testing.T) {
	s := service.NewWithClient(&config.Config{Aliases: map[string]config.Alias{}}, nil)
	if _, err := s.AliasInfo("missing"); err == nil {
		t.Fatalf("expected error for unknown alias")
	}
}

func TestEnsureImageErrors(t *testing.T) {
	s := service.NewWithClient(&config.Config{Aliases: map[string]config.Alias{}}, nil)
	if _, err := s.EnsureImage(context.Background(), "missing", nil); err == nil {
		t.Fatalf("expected error for unknown alias")
	}

	s = service.NewWithClient(&config.Config{Aliases: map[string]config.Alias{
		"demo": {},
	}}, nil)
	if _, err := s.EnsureImage(context.Background(), "demo", nil); err == nil {
		t.Fatalf("expected error for alias with no image")
	}
}

func TestNormalizeImageRef(t *testing.T) {
	got := service.NormalizeImageRef("  ubuntu:24.04  ")
	if got != "ubuntu:24.04" {
		t.Fatalf("unexpected ref: %q", got)
	}
}
