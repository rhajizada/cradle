package cli_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/rhajizada/cradle/internal/cli"
)

func TestRootCommandWiring(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := cli.NewRootCmd("test", log)

	if root.Use != "cradle" {
		t.Fatalf("unexpected root Use: %q", root.Use)
	}

	sub := map[string]bool{}
	for _, c := range root.Commands() {
		sub[c.Name()] = true
	}

	for _, name := range []string{"build", "ls", "run", "stop"} {
		if !sub[name] {
			t.Fatalf("missing subcommand %q", name)
		}
	}
}

func TestCommandBuilders(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := ""

	if got := cli.NewBuildCmd(&cfg, log).Use; got == "" {
		t.Fatalf("build command Use is empty")
	}
	if got := cli.NewLsCmd(&cfg, log).Use; got == "" {
		t.Fatalf("ls command Use is empty")
	}
	if got := cli.NewRunCmd(&cfg, log).Use; got == "" {
		t.Fatalf("run command Use is empty")
	}
	if got := cli.NewStopCmd(&cfg, log).Use; got == "" {
		t.Fatalf("stop command Use is empty")
	}
}

func TestRootCommandVersionAndHelp(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := cli.NewRootCmd("test", log)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)

	root.SetArgs([]string{"-V"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version execute error: %v", err)
	}

	root = cli.NewRootCmd("test", log)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{})
	if err := root.Execute(); err != nil {
		t.Fatalf("help execute error: %v", err)
	}
}
