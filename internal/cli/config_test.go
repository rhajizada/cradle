package cli_test

import (
	"path/filepath"
	"testing"

	"github.com/rhajizada/cradle/internal/cli"
)

func TestDefaultConfigPathXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got := cli.DefaultConfigPath()
	want := filepath.Join("/tmp/xdg", "cradle", "config.yaml")
	if got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
}

func TestDefaultConfigPathHomeFallback(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home")
	got := cli.DefaultConfigPath()
	want := filepath.Join("/tmp/home", ".config", "cradle", "config.yaml")
	if got != want {
		t.Fatalf("unexpected path: got %q want %q", got, want)
	}
}
