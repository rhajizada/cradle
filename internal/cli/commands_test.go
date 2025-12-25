package cli

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAppLoadsConfig(t *testing.T) {
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
      cmd: ["bash"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newApp(cfgPath, log)
	if err != nil {
		t.Fatalf("newApp error: %v", err)
	}
	if app.cfg == nil || app.svc == nil || app.renderer == nil {
		t.Fatalf("expected app context to be populated")
	}
	_ = app.svc.Close()
}

func TestCommandRunEConfigError(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfgPath := "/nonexistent/config.yaml"

	buildCmd := newBuildCmd(&cfgPath, log)
	if err := buildCmd.RunE(buildCmd, []string{"all"}); err == nil {
		t.Fatalf("expected build command to fail with bad config path")
	}

	lsCmd := newLsCmd(&cfgPath, log)
	if err := lsCmd.RunE(lsCmd, nil); err == nil {
		t.Fatalf("expected ls command to fail with bad config path")
	}

	runCmd := newRunCmd(&cfgPath, log)
	if err := runCmd.RunE(runCmd, []string{"demo"}); err == nil {
		t.Fatalf("expected run command to fail with bad config path")
	}

	stopCmd := newStopCmd(&cfgPath, log)
	if err := stopCmd.RunE(stopCmd, []string{"demo"}); err == nil {
		t.Fatalf("expected stop command to fail with bad config path")
	}
}
