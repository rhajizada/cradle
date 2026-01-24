package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rhajizada/cradle/internal/config"
)

func TestLoadFileResolvesPaths(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      build:
        cwd: ./images/demo
    run:
      mounts:
        - type: bind
          source: ./src
          target: /workspace
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := config.LoadFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadFile error: %v", err)
	}

	a := cfg.Aliases["demo"]
	if a.Image.Build == nil {
		t.Fatalf("expected build spec")
	}
	wantCwd := filepath.Join(dir, "images", "demo")
	if a.Image.Build.Cwd != wantCwd {
		t.Fatalf("cwd not resolved: got %q want %q", a.Image.Build.Cwd, wantCwd)
	}

	if len(a.Run.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(a.Run.Mounts))
	}
	wantSrc := filepath.Join(dir, "src")
	if a.Run.Mounts[0].Source != wantSrc {
		t.Fatalf("mount source not resolved: got %q want %q", a.Run.Mounts[0].Source, wantSrc)
	}
}

func TestLoadFileUnknownField(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
version: 1
aliases:
  demo:
    image:
      pull:
        ref: ubuntu:24.04
    run:
      bogus: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := config.LoadFile(cfgPath); err == nil {
		t.Fatalf("expected error for unknown field")
	}
}

func TestValidateUIDGID(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}},
				Run:   config.RunSpec{UID: 1000},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error when gid missing")
	}
}

func TestValidateMountType(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Pull: &config.PullSpec{Ref: "ubuntu:24.04"}},
				Run: config.RunSpec{
					Mounts: []config.MountSpec{{Type: "bad", Target: "/x"}},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for invalid mount type")
	}
}

func TestValidateImageSpecErrors(t *testing.T) {
	cfg := &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing image spec")
	}

	cfg = &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{
					Pull:  &config.PullSpec{Ref: "ubuntu:24.04"},
					Build: &config.BuildSpec{Cwd: "/tmp"},
				},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for pull+build")
	}

	cfg = &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Pull: &config.PullSpec{Ref: ""}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for empty pull ref")
	}

	cfg = &config.Config{
		Aliases: map[string]config.Alias{
			"demo": {
				Image: config.ImageSpec{Build: &config.BuildSpec{Cwd: ""}},
			},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for missing build cwd")
	}
}
