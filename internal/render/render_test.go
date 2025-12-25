package render

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/rhajizada/cradle/internal/service"
)

func TestListStatusesTable(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := New(log, &buf)

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
		if l := len(imageStatusLabel(item.ImagePresent)); l > maxImageStatus {
			maxImageStatus = l
		}
		if l := len(containerStatusLabel(item)); l > maxContainerStatus {
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
	for _, item := range items {
		expected += fmt.Sprintf(
			"%-*s  %-*s  %-*s  %-*s  %-*s\n",
			maxName, item.Name,
			maxImageRef, item.ImageRef,
			maxImageStatus, imageStatusLabel(item.ImagePresent),
			maxContainerName, item.ContainerName,
			maxContainerStatus, containerStatusLabel(item),
		)
	}

	if buf.String() != expected {
		t.Fatalf("unexpected output:\n%s", buf.String())
	}
}

func TestRunStartStopAndBuildStart(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := New(log, io.Discard)

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
		if got := containerStatusLabel(item); got != want {
			t.Fatalf("status %q: got %q want %q", status, got, want)
		}
	}

	item := service.AliasStatus{ContainerPresent: false}
	if got := containerStatusLabel(item); got != "❌" {
		t.Fatalf("missing container: got %q", got)
	}
}
