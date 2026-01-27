package logging_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rhajizada/cradle/internal/logging"
)

func TestHandlerFormatsMessage(t *testing.T) {
	var buf bytes.Buffer
	h := logging.NewHandler(&buf, slog.LevelInfo)
	logger := slog.New(h)

	logger.Info("hello", "user", "alice")

	got := buf.String()
	if got == "" {
		t.Fatalf("expected output")
	}
	if want := "✅ INFO hello user=alice\n"; got != want {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestHandlerGroupsAndLevels(t *testing.T) {
	var buf bytes.Buffer
	h := logging.NewHandler(&buf, slog.LevelDebug)
	logger := slog.New(h.WithGroup("grp"))

	logger.Debug("dbg", "k", "v")
	if got := buf.String(); got != "🐛 DEBUG dbg grp.k=v\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := logging.NewHandler(&buf, slog.LevelInfo)
	logger := slog.New(h.WithAttrs([]slog.Attr{slog.String("app", "cradle")}))

	logger.Info("hello")
	if got := buf.String(); got != "✅ INFO hello app=cradle\n" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestWithGroupEmptyReturnsSame(t *testing.T) {
	h := logging.NewHandler(io.Discard, slog.LevelInfo)
	if got := h.WithGroup(""); got != h {
		t.Fatalf("expected WithGroup(\"\") to return same handler")
	}
}

func TestAppendAttrsColor(t *testing.T) {
	var b strings.Builder
	logging.AppendAttrs(&b, []slog.Attr{slog.String("k", "v")}, nil, true)
	got := b.String()
	if !strings.Contains(got, "k") || !strings.Contains(got, "v") {
		t.Fatalf("expected colored attrs to contain key and value, got %q", got)
	}
}

func TestFormatValueString(t *testing.T) {
	if got := logging.FormatValue(slog.StringValue("x")); got != "x" {
		t.Fatalf("unexpected string format: %q", got)
	}
}

func TestFormatValueTime(t *testing.T) {
	ts := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	got := logging.FormatValue(slog.TimeValue(ts))
	want := "2025-01-02T03:04:05Z"
	if got != want {
		t.Fatalf("unexpected time format: %q", got)
	}
}

func TestEnabled(t *testing.T) {
	h := logging.NewHandler(io.Discard, slog.LevelWarn)
	if h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatalf("expected info to be disabled")
	}
	if !h.Enabled(context.Background(), slog.LevelError) {
		t.Fatalf("expected error to be enabled")
	}
}

func TestLevelStyle(t *testing.T) {
	emoji, label, _ := logging.LevelStyle(slog.LevelError)
	if emoji != "❌" || label != "ERROR" {
		t.Fatalf("unexpected error style: %q %q", emoji, label)
	}
	emoji, label, _ = logging.LevelStyle(slog.LevelWarn)
	if emoji != "⚠️" || label != "WARN" {
		t.Fatalf("unexpected warn style: %q %q", emoji, label)
	}
	emoji, label, _ = logging.LevelStyle(slog.LevelInfo)
	if emoji != "✅" || label != "INFO" {
		t.Fatalf("unexpected info style: %q %q", emoji, label)
	}
	emoji, label, _ = logging.LevelStyle(slog.LevelDebug)
	if emoji != "🐛" || label != "DEBUG" {
		t.Fatalf("unexpected debug style: %q %q", emoji, label)
	}
}

func TestIsTerminalRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if logging.IsTerminal(os.Stdout) {
		t.Fatalf("expected NO_COLOR to disable terminal colors")
	}
}

func TestIsTerminalNonFD(t *testing.T) {
	if logging.IsTerminal(&bytes.Buffer{}) {
		t.Fatalf("expected non-fd writer to be non-terminal")
	}
}
