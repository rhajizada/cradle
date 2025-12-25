package render

import (
	"log/slog"

	"github.com/rhajizada/cradle/internal/service"
)

type Renderer struct {
	log *slog.Logger
}

func New(log *slog.Logger) *Renderer {
	return &Renderer{log: log}
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
	for _, item := range items {
		r.log.Info(
			"alias",
			"name", item.Name,
			"kind", item.Kind,
			"image", item.ImageRef,
			"image_present", item.ImagePresent,
			"container", item.ContainerName,
			"container_present", item.ContainerPresent,
			"container_status", item.ContainerStatus,
		)
	}
}

func (r *Renderer) RunStart(id string) {
	r.log.Info("container started", "id", id)
}

func (r *Renderer) RunStop(id string) {
	r.log.Info("container stopped", "id", id)
}
