package render_test

import (
	"bytes"
	"fmt"
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

	maxName := len("ALIAS")
	maxImageRef := len("IMAGE")
	maxImageStatus := len("STATUS")
	maxContainerName := len("CONTAINER")
	maxContainerStatus := len("STATUS")
	for _, item := range items {
		if len(item.Name) > maxName {
			maxName = len(item.Name)
		}
		if len(item.ImageRef) > maxImageRef {
			maxImageRef = len(item.ImageRef)
		}
		if len(item.ContainerName) > maxContainerName {
			maxContainerName = len(item.ContainerName)
		}
		if l := len(render.ImageStatusLabel(item.ImagePresent)); l > maxImageStatus {
			maxImageStatus = l
		}
		if l := len(render.ContainerStatusLabel(item)); l > maxContainerStatus {
			maxContainerStatus = l
		}
	}

	expected := ""
	expected += fmt.Sprintf(
		"%-*s  %-*s  %-*s  %-*s  %-*s\n",
		maxName, "ALIAS",
		maxImageRef, "IMAGE",
		maxImageStatus, "STATUS",
		maxContainerName, "CONTAINER",
		maxContainerStatus, "STATUS",
	)
	expected += fmt.Sprintf(
		"%-*s  %-*s  %-*s  %-*s  %-*s\n",
		maxName, strings.Repeat("-", maxName),
		maxImageRef, strings.Repeat("-", maxImageRef),
		maxImageStatus, strings.Repeat("-", maxImageStatus),
		maxContainerName, strings.Repeat("-", maxContainerName),
		maxContainerStatus, strings.Repeat("-", maxContainerStatus),
	)
	var expectedSb88 strings.Builder
	for _, item := range items {
		expectedSb88.WriteString(fmt.Sprintf(
			"%-*s  %-*s  %-*s  %-*s  %-*s\n",
			maxName, item.Name,
			maxImageRef, item.ImageRef,
			maxImageStatus, render.ImageStatusLabel(item.ImagePresent),
			maxContainerName, item.ContainerName,
			maxContainerStatus, render.ContainerStatusLabel(item),
		))
	}
	expected += expectedSb88.String()

	if buf.String() != expected {
		t.Fatalf("unexpected output:\n%s", buf.String())
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
