package service

import (
	"context"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
)

func TestAliasInfoAndListAliases(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"b": {Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}}},
			"a": {Image: config.ImageSpec{Build: &config.BuildSpec{Cwd: "/tmp"}}},
		},
	}

	s := &Service{cfg: cfg}

	infos := s.ListAliases()
	if len(infos) != 2 || infos[0].Name != "a" || infos[1].Name != "b" {
		t.Fatalf("expected sorted aliases, got %+v", infos)
	}

	info, err := s.AliasInfo("b")
	if err != nil {
		t.Fatalf("AliasInfo error: %v", err)
	}
	if info.Kind != ImagePull || info.Ref != "ubuntu:24.04" {
		t.Fatalf("unexpected pull info: %+v", info)
	}
	if info.Description() != "pull: ubuntu:24.04" {
		t.Fatalf("unexpected description: %q", info.Description())
	}

	info, err = s.AliasInfo("a")
	if err != nil {
		t.Fatalf("AliasInfo error: %v", err)
	}
	if info.Kind != ImageBuild || info.Tag == "" {
		t.Fatalf("unexpected build info: %+v", info)
	}
	if info.Description() == "" {
		t.Fatalf("expected description for build")
	}
}

func TestAliasInfoErrors(t *testing.T) {
	s := &Service{cfg: &config.Config{Aliases: map[string]config.Alias{}}}
	if _, err := s.AliasInfo("missing"); err == nil {
		t.Fatalf("expected error for unknown alias")
	}
}

func TestEnsureImageErrors(t *testing.T) {
	s := &Service{cfg: &config.Config{Aliases: map[string]config.Alias{}}}
	if _, err := s.ensureImage(context.Background(), "missing", nil); err == nil {
		t.Fatalf("expected error for unknown alias")
	}

	s = &Service{cfg: &config.Config{Aliases: map[string]config.Alias{
		"demo": {},
	}}}
	if _, err := s.ensureImage(context.Background(), "demo", nil); err == nil {
		t.Fatalf("expected error for alias with no image")
	}
}

func TestNormalizeImageRef(t *testing.T) {
	got := normalizeImageRef("  ubuntu:24.04  ")
	if got != "ubuntu:24.04" {
		t.Fatalf("unexpected ref: %q", got)
	}
}
