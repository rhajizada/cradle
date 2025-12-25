package render

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/rhajizada/cradle/internal/service"
)

type Renderer struct {
	log *slog.Logger
	out io.Writer
}

func New(log *slog.Logger, out io.Writer) *Renderer {
	return &Renderer{log: log, out: out}
}

func (r *Renderer) BuildStart(info service.AliasInfo) {
	switch info.Kind {
	case service.ImagePull:
		r.log.Info("image pull", "ref", info.Ref)
	case service.ImageBuild:
		r.log.Info("image build", "tag", info.Tag, "context", info.Cwd)
	}
}

func (r *Renderer) ListStatuses(items []service.AliasStatus) {
	if len(items) == 0 {
		fmt.Fprintln(r.out, "No aliases found.")
		return
	}

	maxName := len("ALIAS")
	maxImage := len("IMAGE")
	maxContainer := len("CONTAINER")
	for _, item := range items {
		if len(item.Name) > maxName {
			maxName = len(item.Name)
		}
		image := fmt.Sprintf("%s %s", item.ImageRef, imageStatusLabel(item.ImagePresent))
		if len(image) > maxImage {
			maxImage = len(image)
		}
		container := fmt.Sprintf("%s %s", item.ContainerName, containerStatusLabel(item))
		if len(container) > maxContainer {
			maxContainer = len(container)
		}
	}

	header := fmt.Sprintf(
		"%-*s  %-*s  %-*s\n",
		maxName, "ALIAS",
		maxImage, "IMAGE",
		maxContainer, "CONTAINER",
	)
	divider := fmt.Sprintf(
		"%-*s  %-*s  %-*s\n",
		maxName, strings.Repeat("-", maxName),
		maxImage, strings.Repeat("-", maxImage),
		maxContainer, strings.Repeat("-", maxContainer),
	)

	fmt.Fprint(r.out, header)
	fmt.Fprint(r.out, divider)
	for _, item := range items {
		fmt.Fprintf(
			r.out,
			"%-*s  %-*s  %-*s\n",
			maxName,
			item.Name,
			maxImage,
			fmt.Sprintf("%s %s", item.ImageRef, imageStatusLabel(item.ImagePresent)),
			maxContainer,
			fmt.Sprintf("%s %s", item.ContainerName, containerStatusLabel(item)),
		)
	}
}

func (r *Renderer) RunStart(id string) {
	r.log.Info("container started", "id", id)
}

func (r *Renderer) RunStop(id string) {
	r.log.Info("container stopped", "id", id)
}

func imageStatusLabel(present bool) string {
	if present {
		return "✅"
	}
	return "❌"
}

func containerStatusLabel(item service.AliasStatus) string {
	if !item.ContainerPresent {
		return "❌"
	}
	if item.ContainerStatus == "" {
		return "🤷"
	}
	switch item.ContainerStatus {
	case "running":
		return "🟢"
	case "exited":
		return "🟡"
	case "created":
		return "⚪"
	case "paused":
		return "⏸️"
	case "restarting":
		return "🔄"
	case "dead":
		return "💀"
	default:
		return "🤷"
	}
}
