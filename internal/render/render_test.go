package render_test

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/rhajizada/cradle/internal/render"
	"github.com/rhajizada/cradle/internal/service"
)

func TestListStatusesTable(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := render.New(log, &buf)

	items := []service.AliasStatus{
		{
			Name:             "web",
			ImageRef:         "node:22",
			ImagePresent:     true,
			ContainerName:    "cradle-web",
			ContainerPresent: true,
			ContainerStatus:  "running",
		},
		{
			Name:             "db",
			ImageRef:         "postgres:16",
			ImagePresent:     false,
			ContainerName:    "cradle-db",
			ContainerPresent: false,
			ContainerStatus:  "",
		},
		{
			Name:             "paused",
			ImageRef:         "busybox:1",
			ImagePresent:     true,
			ContainerName:    "cradle-paused",
			ContainerPresent: true,
			ContainerStatus:  "paused",
		},
	}

	r.ListStatuses(items)

	out := buf.String()

	// Basic structure: header labels should be present.
	for _, s := range []string{"Alias", "Image", "Status", "Container"} {
		if !strings.Contains(out, s) {
			t.Fatalf("expected %q in output:\n%s", s, out)
		}
	}

	// Rows: key fields.
	for _, s := range []string{
		"web", "node:22", "cradle-web",
		"db", "postgres:16", "cradle-db",
		"paused", "busybox:1", "cradle-paused",
	} {
		if !strings.Contains(out, s) {
			t.Fatalf("expected %q in output:\n%s", s, out)
		}
	}

	// Image status: emoji + lowercase text.
	if !strings.Contains(out, "✅ present") {
		t.Fatalf("expected image present status in output:\n%s", out)
	}
	if !strings.Contains(out, "❌ missing") {
		t.Fatalf("expected image missing status in output:\n%s", out)
	}

	// Container status: emoji + lowercase text.
	if !strings.Contains(out, "▶️ running") {
		t.Fatalf("expected running container status in output:\n%s", out)
	}
	if !strings.Contains(out, "⏸️ paused") {
		t.Fatalf("expected paused container status in output:\n%s", out)
	}
	// For ContainerPresent=false, we expect the "missing" text (paired with ❌).
	if !strings.Contains(out, "❌ missing") {
		t.Fatalf("expected missing container status in output:\n%s", out)
	}
}

func TestRunStartStopAndBuildStart(_ *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := render.New(log, io.Discard)

	r.RunStart("id")
	r.RunStop("id")

	r.BuildStart(service.AliasInfo{Kind: service.ImagePull, Ref: "ubuntu:24.04"})
	r.BuildStart(service.AliasInfo{Kind: service.ImageBuild, Tag: "cradle/test:latest", Cwd: "/tmp"})
}

func TestContainerStatusLabelVariants(t *testing.T) {
	statuses := map[string]string{
		"running":    "▶️",
		"exited":     "⛔️",
		"created":    "✅",
		"paused":     "⏸️",
		"restarting": "🔄",
		"dead":       "💀",
		"unknown":    "🤷",
		"":           "🤷",
	}
	for status, want := range statuses {
		item := service.AliasStatus{
			ContainerPresent: true,
			ContainerStatus:  status,
		}
		if got := render.ContainerStatusLabel(item); got != want {
			t.Fatalf("status %q: got %q want %q", status, got, want)
		}
	}

	item := service.AliasStatus{ContainerPresent: false}
	if got := render.ContainerStatusLabel(item); got != "❌" {
		t.Fatalf("missing container: got %q", got)
	}
}
